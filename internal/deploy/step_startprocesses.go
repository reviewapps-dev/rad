package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/reviewapps-dev/rad/internal/app"
	"github.com/reviewapps-dev/rad/internal/process"
	"github.com/reviewapps-dev/rad/internal/rv"
)

type StartProcessesStep struct{}

func (s *StartProcessesStep) Name() string { return "start-processes" }

func (s *StartProcessesStep) Run(ctx *StepContext) error {
	_ = ctx.Store.UpdateStatus(ctx.AppState.AppID, app.StatusStarting, "")

	procs := ctx.Processes
	if len(procs) == 0 {
		// Default: just the web process (rails server)
		procs = map[string]string{
			"web": fmt.Sprintf("bin/rails server -p %d -e production", ctx.Port),
		}
	}

	// On redeploy, stop ALL old processes first
	if ctx.Redeploy && len(ctx.AppState.Processes) > 0 {
		ctx.Logger.Log("stopping %d old process(es)", len(ctx.AppState.Processes))
		stopAllProcesses(ctx)
	} else if ctx.Redeploy && ctx.AppState.PID > 0 {
		// Backward compat: single PID from before multi-process
		ctx.Logger.Log("stopping old process (pid=%d)", ctx.AppState.PID)
		process.Stop(ctx.AppState.PID)
	}

	// Clear old process info before starting fresh
	_ = ctx.Store.ClearProcesses(ctx.AppState.AppID)

	// Start web process first, then others in sorted order
	sortedNames := sortProcessNames(procs)

	for _, name := range sortedNames {
		cmd := procs[name]

		// Expand $PORT in the command for the web process
		if name == "web" {
			cmd = strings.ReplaceAll(cmd, "$PORT", fmt.Sprintf("%d", ctx.Port))
		}

		ctx.Logger.Log("starting process %q: %s", name, cmd)

		logPath := processLogPath(ctx, name)
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return fmt.Errorf("open log file for %s: %w", name, err)
		}

		execCmd := rv.ExecInDir(ctx.RepoDir, ctx.AppState.RubyVersion, buildEnvSlice(ctx.EnvMap), cmd)
		execCmd.Stdout = logFile
		execCmd.Stderr = logFile

		info, err := process.Start(execCmd)
		if err != nil {
			logFile.Close()
			// If a non-web process fails to start, stop everything we already started
			ctx.Logger.Log("process %q failed to start: %v", name, err)
			stopAllProcesses(ctx)
			return fmt.Errorf("start process %s: %w", name, err)
		}

		proc := app.ProcessInfo{
			Name: name,
			PID:  info.PID,
		}
		if name == "web" {
			proc.Port = ctx.Port
			ctx.PID = info.PID
			ctx.AppState.PID = info.PID
		}

		_ = ctx.Store.SetProcess(ctx.AppState.AppID, proc)
		if ctx.AppState.Processes == nil {
			ctx.AppState.Processes = make(map[string]app.ProcessInfo)
		}
		ctx.AppState.Processes[name] = proc

		ctx.Logger.Log("process %q started (pid=%d, log=%s)", name, info.PID, logPath)
	}

	// Save the process commands so restart can re-use them
	ctx.AppState.ProcessCommands = procs

	ctx.Logger.Log("all %d process(es) started", len(procs))
	return nil
}

// sortProcessNames returns process names with "web" first, then the rest alphabetically.
func sortProcessNames(procs map[string]string) []string {
	names := make([]string, 0, len(procs))
	for name := range procs {
		if name != "web" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	// web always goes first
	if _, ok := procs["web"]; ok {
		names = append([]string{"web"}, names...)
	}
	return names
}

// processLogPath returns the log file path for a given process name.
// web → {app_id}.log, others → {app_id}.{name}.log
func processLogPath(ctx *StepContext, name string) string {
	if name == "web" {
		return filepath.Join(ctx.Config.Paths.LogDir, ctx.AppState.AppID+".log")
	}
	return filepath.Join(ctx.Config.Paths.LogDir, ctx.AppState.AppID+"."+name+".log")
}

// stopAllProcesses stops all tracked processes for the app.
func stopAllProcesses(ctx *StepContext) {
	if len(ctx.AppState.Processes) == 0 {
		return
	}
	for name, proc := range ctx.AppState.Processes {
		if proc.PID > 0 {
			ctx.Logger.Log("stopping process %q (pid=%d)", name, proc.PID)
			if err := process.Stop(proc.PID); err != nil {
				ctx.Logger.Log("failed to stop process %q (pid=%d): %v", name, proc.PID, err)
			}
		}
	}
}
