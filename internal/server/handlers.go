package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/reviewapps-dev/rad/internal/app"
	"github.com/reviewapps-dev/rad/internal/buildqueue"
	"github.com/reviewapps-dev/rad/internal/callback"
	"github.com/reviewapps-dev/rad/internal/database"
	"github.com/reviewapps-dev/rad/internal/deploy"
	"github.com/reviewapps-dev/rad/internal/fnm"
	"github.com/reviewapps-dev/rad/internal/process"
	"github.com/reviewapps-dev/rad/internal/reviewappsyml"
	"github.com/reviewapps-dev/rad/internal/rv"
	"github.com/reviewapps-dev/rad/internal/updater"
	"github.com/reviewapps-dev/rad/internal/version"
)

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	rvVersion := "unknown"
	if rvBin, err := exec.LookPath("rv"); err == nil {
		if out, err := exec.Command(rvBin, "--version").Output(); err == nil {
			rvVersion = strings.TrimSpace(string(out))
		}
	}

	fnmVersion := "unknown"
	if out, err := exec.Command("fnm", "--version").Output(); err == nil {
		fnmVersion = strings.TrimSpace(string(out))
	}

	caddyVersion := "not installed"
	if out, err := exec.Command("caddy", "version").Output(); err == nil {
		fields := strings.Fields(strings.TrimSpace(string(out)))
		if len(fields) > 0 {
			caddyVersion = fields[0]
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":        "ok",
		"version":       version.Version,
		"commit":        version.Commit,
		"uptime":        time.Since(s.startTime).Seconds(),
		"app_count":     s.store.Count(),
		"rv_version":    rvVersion,
		"fnm_version":   fnmVersion,
		"caddy_version": caddyVersion,
		"ruby_versions": rv.ListInstalled(),
		"node_versions": fnm.ListInstalled(),
	})
}

func (s *Server) handleListApps(w http.ResponseWriter, r *http.Request) {
	apps := s.store.List()
	writeJSON(w, http.StatusOK, map[string]any{
		"apps": apps,
	})
}

func (s *Server) handleGetAppStatus(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	state, err := s.store.Get(appID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// Build the app URL
	url := ""
	if state.Port > 0 {
		url = fmt.Sprintf("http://localhost:%d", state.Port)
		if state.Subdomain != "" && !s.cfg.Dev {
			url = fmt.Sprintf("https://%s.srv.reviewapps.dev", state.Subdomain)
		}
	}

	// Compute uptime since deploy
	var uptimeSecs float64
	if state.Status == app.StatusRunning {
		uptimeSecs = time.Since(state.UpdatedAt).Seconds()
	}

	// Get memory usage of web process (best-effort)
	var memoryMB int
	if state.PID > 0 {
		memoryMB = getProcessMemoryMB(state.PID)
	}

	// Build enriched response (includes all AppState fields plus computed ones)
	resp := map[string]any{
		"app_id":           state.AppID,
		"repo_url":         state.RepoURL,
		"branch":           state.Branch,
		"commit_sha":       state.CommitSHA,
		"ruby_version":     state.RubyVersion,
		"node_version":     state.NodeVersion,
		"database_adapter": state.DatabaseAdapter,
		"subdomain":        state.Subdomain,
		"status":           state.Status,
		"port":             state.Port,
		"pid":              state.PID,
		"url":              url,
		"memory_mb":        memoryMB,
		"uptime":           uptimeSecs,
		"processes":        state.Processes,
		"error":            state.Error,
		"created_at":       state.CreatedAt,
		"updated_at":       state.UpdatedAt,
		"build_log":        state.BuildLog,
	}

	writeJSON(w, http.StatusOK, resp)
}

// getProcessMemoryMB returns the RSS memory in MB for a given PID.
func getProcessMemoryMB(pid int) int {
	out, err := exec.Command("ps", "-o", "rss=", "-p", fmt.Sprintf("%d", pid)).Output()
	if err != nil {
		return 0
	}
	// ps returns RSS in kilobytes
	rssKB := strings.TrimSpace(string(out))
	var kb int
	if _, err := fmt.Sscanf(rssKB, "%d", &kb); err != nil {
		return 0
	}
	return kb / 1024
}

func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request) {
	var req DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.AppID == "" {
		writeError(w, http.StatusBadRequest, "app_id is required")
		return
	}
	if req.RepoURL == "" {
		writeError(w, http.StatusBadRequest, "repo_url is required")
		return
	}
	if req.Branch == "" {
		req.Branch = "main"
	}
	if req.RubyVersion == "" {
		req.RubyVersion = s.cfg.Defaults.RubyVersion
	}
	if req.DatabaseAdapter == "" {
		req.DatabaseAdapter = s.cfg.Defaults.DatabaseAdapter
	}

	// Detect if this is a redeploy (app already exists and is running)
	isRedeploy := false
	if existing, err := s.store.Get(req.AppID); err == nil {
		isRedeploy = true
		log.Printf("deploy: redeploy for %s (status=%s, pid=%d)", req.AppID, existing.Status, existing.PID)
	}

	// Convert request hooks to app hooks
	var hooks *app.Hooks
	if req.Hooks != nil {
		hooks = &app.Hooks{
			AfterClone:     req.Hooks.AfterClone,
			BeforeBuild:    req.Hooks.BeforeBuild,
			AfterBuild:     req.Hooks.AfterBuild,
			BeforeMigrate:  req.Hooks.BeforeMigrate,
			AfterDeploy:    req.Hooks.AfterDeploy,
			BeforeTeardown: req.Hooks.BeforeTeardown,
			OnFailure:      req.Hooks.OnFailure,
		}
	}

	state := &app.AppState{
		AppID:           req.AppID,
		RepoURL:         req.RepoURL,
		Branch:          req.Branch,
		CommitSHA:       req.CommitSHA,
		RubyVersion:     req.RubyVersion,
		NodeVersion:     req.NodeVersion,
		DatabaseAdapter: req.DatabaseAdapter,
		Databases:       req.Databases,
		EnvVars:         req.EnvVars,
		BuildCommand:    req.BuildCommand,
		SeedCommand:     req.SeedCommand,
		Subdomain:       req.Subdomain,
		CallbackURL:     req.CallbackURL,
		Hooks:           hooks,
		Status:          app.StatusQueued,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	s.store.Put(state)

	redeploy := isRedeploy
	ok := s.queue.Enqueue(buildqueue.Job{
		AppID: req.AppID,
		Fn: func(ctx context.Context) error {
			if s.deployFn != nil {
				return s.deployFn(ctx, state, redeploy)
			}
			log.Printf("deploy: no deploy function set, skipping %s", req.AppID)
			return nil
		},
	})

	if !ok {
		writeError(w, http.StatusServiceUnavailable, "build queue full")
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "queued",
		"app_id":  req.AppID,
		"message": "deploy queued",
	})
}

func (s *Server) handleTeardown(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	state, err := s.store.Get(appID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	_ = s.store.UpdateStatus(appID, app.StatusTeardown, "")

	// Run before_teardown hooks (best-effort)
	if state.AppDir != "" {
		repoDir := filepath.Join(state.AppDir, "repo")
		ymlPath := filepath.Join(repoDir, "reviewapps.yml")
		if _, err := os.Stat(ymlPath); err == nil {
			if cfg, err := reviewappsyml.Parse(ymlPath); err == nil {
				envSlice := s.loadAppEnv(state)
				log.Printf("teardown: running before_teardown hooks for %s", appID)
				if err := deploy.RunHooksFromConfig(cfg, deploy.HookBeforeTeardown, repoDir, state.RubyVersion, envSlice); err != nil {
					log.Printf("teardown: before_teardown hook error (non-fatal): %v", err)
				}
			}
		}
	}

	// Stop all processes
	if len(state.Processes) > 0 {
		log.Printf("teardown: stopping %d process(es) for %s", len(state.Processes), appID)
		for name, proc := range state.Processes {
			if proc.PID > 0 {
				log.Printf("teardown: stopping %s (pid=%d)", name, proc.PID)
				if err := process.Stop(proc.PID); err != nil {
					log.Printf("teardown: stop %s: %v", name, err)
				}
			}
		}
	} else if state.PID > 0 {
		// Backward compat: single PID from before multi-process
		log.Printf("teardown: stopping process %d for %s", state.PID, appID)
		if err := process.Stop(state.PID); err != nil {
			log.Printf("teardown: stop process: %v", err)
		}
	}

	// Release port
	s.ports.Release(appID)

	// Drop PostgreSQL databases
	databases := state.Databases
	if len(databases) == 0 && (state.DatabaseAdapter == "postgresql" || state.DatabaseAdapter == "postgres") {
		databases = map[string]string{"primary": state.DatabaseAdapter}
	}
	for name, adapter := range databases {
		if adapter == "postgresql" || adapter == "postgres" {
			dbCfg := &database.DBConfig{AppID: appID, Name: name, Adapter: adapter}
			dbName := dbCfg.DBName()
			log.Printf("teardown: dropping database %s", dbName)
			if err := database.DropPostgresDB(dbName); err != nil {
				log.Printf("teardown: dropdb %s: %v", dbName, err)
			}
		}
	}

	// Remove Caddy site config and reload
	if s.caddy != nil {
		if err := s.caddy.RemoveSiteConfig(appID); err != nil {
			log.Printf("teardown: caddy remove: %v", err)
		}
		if err := s.caddy.Reload(); err != nil {
			log.Printf("teardown: caddy reload: %v", err)
		}
	}

	// Remove log files ({app_id}.log, {app_id}.*.log, {app_id}.access.log)
	logPattern := filepath.Join(s.cfg.Paths.LogDir, appID+"*.log")
	if matches, err := filepath.Glob(logPattern); err == nil {
		for _, f := range matches {
			log.Printf("teardown: removing log %s", f)
			os.Remove(f)
		}
	}

	// Remove app directory
	if state.AppDir != "" {
		log.Printf("teardown: removing %s", state.AppDir)
		os.RemoveAll(state.AppDir)
	}

	if err := s.store.Delete(appID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Send teardown callback to web app
	if state.CallbackURL != "" {
		client := callback.NewClient()
		client.SendStatus(state.CallbackURL, callback.StatusPayload{
			AppID:  appID,
			Status: "removed",
		})
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "removed", "app_id": appID})
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	state, err := s.store.Get(appID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if state.Status != app.StatusRunning {
		writeError(w, http.StatusConflict, "app is not running (status: "+string(state.Status)+")")
		return
	}

	_ = s.store.UpdateStatus(appID, app.StatusStarting, "")

	// Stop all existing processes
	if len(state.Processes) > 0 {
		log.Printf("restart: stopping %d process(es) for %s", len(state.Processes), appID)
		for name, proc := range state.Processes {
			if proc.PID > 0 {
				log.Printf("restart: stopping %s (pid=%d)", name, proc.PID)
				process.Stop(proc.PID)
			}
		}
	} else if state.PID > 0 {
		log.Printf("restart: stopping pid=%d for %s", state.PID, appID)
		process.Stop(state.PID)
	}

	// Clear old process info
	_ = s.store.ClearProcesses(appID)

	// Re-read the .env file to build env slice
	envSlice := s.loadAppEnv(state)

	// Re-start all processes using saved commands
	procs := state.ProcessCommands
	if len(procs) == 0 {
		// Fallback: just start rails server
		procs = map[string]string{
			"web": fmt.Sprintf("bin/rails server -p %d -e production", state.Port),
		}
	}

	repoDir := filepath.Join(state.AppDir, "repo")

	for name, cmd := range procs {
		if name == "web" {
			cmd = strings.ReplaceAll(cmd, "$PORT", fmt.Sprintf("%d", state.Port))
		}

		log.Printf("restart: starting %s: %s", name, cmd)

		logPath := s.processLogPath(appID, name)
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Printf("restart: open log: %v", err)
			_ = s.store.UpdateStatus(appID, app.StatusFailed, err.Error())
			writeError(w, http.StatusInternalServerError, "failed to open log file: "+err.Error())
			return
		}

		execCmd := rv.ExecInDir(repoDir, state.RubyVersion, envSlice, cmd)
		execCmd.Stdout = logFile
		execCmd.Stderr = logFile

		info, err := process.Start(execCmd)
		if err != nil {
			logFile.Close()
			log.Printf("restart: start %s failed: %v", name, err)
			_ = s.store.UpdateStatus(appID, app.StatusFailed, err.Error())
			writeError(w, http.StatusInternalServerError, "failed to start process "+name+": "+err.Error())
			return
		}

		proc := app.ProcessInfo{Name: name, PID: info.PID}
		if name == "web" {
			proc.Port = state.Port
		}
		_ = s.store.SetProcess(appID, proc)
		log.Printf("restart: %s started (pid=%d)", name, info.PID)
	}

	_ = s.store.UpdateStatus(appID, app.StatusRunning, "")
	writeJSON(w, http.StatusOK, map[string]string{"status": "restarted", "app_id": appID})
}

func (s *Server) handleExec(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	state, err := s.store.Get(appID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Command == "" {
		writeError(w, http.StatusBadRequest, "command is required")
		return
	}

	repoDir := filepath.Join(state.AppDir, "repo")
	envSlice := s.loadAppEnv(state)

	timeout := 30 * time.Second
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	log.Printf("exec: running %q for %s (timeout=%s)", req.Command, appID, timeout)

	cmd := rv.ExecInDir(repoDir, state.RubyVersion, envSlice, req.Command)

	// Run with timeout using a goroutine
	type result struct {
		output []byte
		err    error
	}
	ch := make(chan result, 1)
	go func() {
		out, err := cmd.CombinedOutput()
		ch <- result{out, err}
	}()

	select {
	case <-ctx.Done():
		// Timeout — kill the process
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		writeJSON(w, http.StatusRequestTimeout, map[string]string{
			"status": "timeout",
			"app_id": appID,
			"error":  fmt.Sprintf("command timed out after %s", timeout),
		})
		return
	case res := <-ch:
		if res.err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"status": "error",
				"app_id": appID,
				"error":  res.err.Error(),
				"output": string(res.output),
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status": "success",
			"app_id": appID,
			"output": string(res.output),
		})
	}
}

// loadAppEnv reads the .env file for an app and returns the env vars as a slice.
// Does NOT include os.Environ() — rv.ExecInDir/RunInDir already prepends that.
func (s *Server) loadAppEnv(state *app.AppState) []string {
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

// processLogPath returns the log file path for a process.
func (s *Server) processLogPath(appID, name string) string {
	if name == "web" {
		return filepath.Join(s.cfg.Paths.LogDir, appID+".log")
	}
	return filepath.Join(s.cfg.Paths.LogDir, appID+"."+name+".log")
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	appID := r.PathValue("app_id")
	state, err := s.store.Get(appID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	logType := r.URL.Query().Get("type")
	if logType == "" {
		logType = "build"
	}

	switch logType {
	case "build":
		// Return build log lines from the store
		writeJSON(w, http.StatusOK, map[string]any{
			"app_id": appID,
			"type":   "build",
			"lines":  state.BuildLog,
		})
	case "runtime":
		// Tail the runtime log file
		processName := r.URL.Query().Get("process")
		if processName == "" {
			processName = "web"
		}

		logPath := s.processLogPath(appID, processName)
		data, err := os.ReadFile(logPath)
		if err != nil {
			writeError(w, http.StatusNotFound, "log file not found: "+err.Error())
			return
		}

		// Return last N lines (default 100)
		lines := strings.Split(string(data), "\n")
		n := 100
		if qn := r.URL.Query().Get("lines"); qn != "" {
			if v, err := strconv.Atoi(qn); err == nil && v > 0 {
				n = v
			}
		}
		if n > len(lines) {
			n = len(lines)
		}
		tail := lines[len(lines)-n:]

		writeJSON(w, http.StatusOK, map[string]any{
			"app_id":  appID,
			"type":    "runtime",
			"process": processName,
			"lines":   tail,
		})
	default:
		writeError(w, http.StatusBadRequest, "invalid type: use 'build' or 'runtime'")
	}
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	var req UpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}

	if req.Version == "" || req.DownloadURL == "" {
		writeError(w, http.StatusBadRequest, "version and download_url are required")
		return
	}

	info := &updater.ReleaseInfo{
		Version:     req.Version,
		DownloadURL: req.DownloadURL,
		Checksum:    req.Checksum,
	}

	// Apply update in background — respond immediately
	go func() {
		log.Printf("update: starting update to %s from %s", info.Version, version.Version)
		if err := updater.Apply(info); err != nil {
			log.Printf("update: failed: %v", err)
			return
		}
		log.Printf("update: installed %s, restarting via systemd", info.Version)

		// Restart via systemd if available
		if updater.IsSystemd() {
			exec.Command("sudo", "systemctl", "restart", "rad").Run()
		}
	}()

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "updating",
		"from":   version.Version,
		"to":     req.Version,
	})
}
