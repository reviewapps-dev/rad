package monitor

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/reviewapps-dev/rad/internal/app"
	"github.com/reviewapps-dev/rad/internal/config"
	"github.com/reviewapps-dev/rad/internal/process"
	"github.com/reviewapps-dev/rad/internal/rv"
)

// Monitor periodically checks running app processes and restarts any that have crashed.
type Monitor struct {
	store    *app.Store
	cfg      *config.Config
	interval time.Duration
	done     chan struct{}
}

func New(cfg *config.Config, store *app.Store, interval time.Duration) *Monitor {
	return &Monitor{
		store:    store,
		cfg:      cfg,
		interval: interval,
		done:     make(chan struct{}),
	}
}

func (m *Monitor) Start() {
	go m.loop()
	log.Printf("monitor: started (interval=%s)", m.interval)
}

func (m *Monitor) Stop() {
	close(m.done)
	log.Printf("monitor: stopped")
}

func (m *Monitor) loop() {
	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.check()
		case <-m.done:
			return
		}
	}
}

func (m *Monitor) check() {
	apps := m.store.List()
	for _, state := range apps {
		if state.Status != app.StatusRunning {
			continue
		}
		if len(state.Processes) == 0 {
			continue
		}

		for name, proc := range state.Processes {
			if proc.PID <= 0 {
				continue
			}
			if isAlive(proc.PID) {
				continue
			}

			log.Printf("monitor: process %s/%s (pid=%d) is dead, restarting", state.AppID, name, proc.PID)
			m.restartProcess(state, name)
		}
	}
}

// isAlive checks if a process is still running by sending signal 0.
func isAlive(pid int) bool {
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil
}

func (m *Monitor) restartProcess(state *app.AppState, name string) {
	cmd, ok := state.ProcessCommands[name]
	if !ok {
		log.Printf("monitor: no saved command for %s/%s, cannot restart", state.AppID, name)
		return
	}

	// Expand $PORT for web process
	if name == "web" && state.Port > 0 {
		cmd = strings.ReplaceAll(cmd, "$PORT", fmt.Sprintf("%d", state.Port))
	}

	repoDir := filepath.Join(state.AppDir, "repo")
	envSlice := m.loadAppEnv(state)

	logPath := m.processLogPath(state.AppID, name)
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Printf("monitor: open log for %s/%s: %v", state.AppID, name, err)
		return
	}

	execCmd := rv.ExecInDir(repoDir, state.RubyVersion, envSlice, cmd)
	execCmd.Stdout = logFile
	execCmd.Stderr = logFile

	info, err := process.Start(execCmd)
	if err != nil {
		logFile.Close()
		log.Printf("monitor: restart %s/%s failed: %v", state.AppID, name, err)
		return
	}

	proc := app.ProcessInfo{Name: name, PID: info.PID}
	if name == "web" {
		proc.Port = state.Port
	}
	_ = m.store.SetProcess(state.AppID, proc)

	log.Printf("monitor: restarted %s/%s (new pid=%d)", state.AppID, name, info.PID)
}

func (m *Monitor) loadAppEnv(state *app.AppState) []string {
	var envSlice []string
	envPath := filepath.Join(state.AppDir, ".env")
	data, err := os.ReadFile(envPath)
	if err != nil {
		return envSlice
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		envSlice = append(envSlice, line)
	}
	return envSlice
}

func (m *Monitor) processLogPath(appID, name string) string {
	if name == "web" {
		return filepath.Join(m.cfg.Paths.LogDir, appID+".log")
	}
	return filepath.Join(m.cfg.Paths.LogDir, appID+"."+name+".log")
}
