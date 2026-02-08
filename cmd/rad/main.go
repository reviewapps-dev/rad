package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/reviewapps-dev/rad/internal/app"
	"github.com/reviewapps-dev/rad/internal/buildqueue"
	"github.com/reviewapps-dev/rad/internal/caddy"
	"github.com/reviewapps-dev/rad/internal/config"
	"github.com/reviewapps-dev/rad/internal/deploy"
	"github.com/reviewapps-dev/rad/internal/heartbeat"
	"github.com/reviewapps-dev/rad/internal/monitor"
	"github.com/reviewapps-dev/rad/internal/port"
	"github.com/reviewapps-dev/rad/internal/server"
	"github.com/reviewapps-dev/rad/internal/updater"
	"github.com/reviewapps-dev/rad/internal/version"
)

func main() {
	// Handle subcommands before flag parsing
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "update":
			runUpdate(os.Args[2:])
			return
		case "version":
			fmt.Printf("rad %s (%s) built %s\n", version.Version, version.Commit, version.BuildDate)
			return
		}
	}

	var (
		dev        = flag.Bool("dev", false, "run in dev mode")
		token      = flag.String("token", "", "auth token (dev mode)")
		listen     = flag.String("listen", "", "override listen address")
		configPath = flag.String("config", "", "path to config.toml")
		showVer    = flag.Bool("version", false, "print version and exit")
	)
	flag.Parse()

	if *showVer {
		fmt.Printf("rad %s (%s) built %s\n", version.Version, version.Commit, version.BuildDate)
		os.Exit(0)
	}

	cfg, err := config.Load(*configPath, *dev)
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	if *token != "" {
		cfg.Auth.Token = *token
	}
	if *listen != "" {
		cfg.Server.Listen = *listen
	}

	if err := cfg.EnsureDirs(); err != nil {
		log.Fatalf("dirs: %v", err)
	}

	statePath := filepath.Join(cfg.Paths.AppsDir, "..", "state.json")
	store := app.NewStore(statePath)
	ports := port.NewAllocator()
	queue := buildqueue.New(100)

	// Recover port assignments from persisted state
	for _, a := range store.List() {
		if a.Port > 0 {
			ports.Reserve(a.AppID, a.Port)
			log.Printf("recovered: %s (status=%s, port=%d)", a.AppID, a.Status, a.Port)
		}
	}

	// Initialize Caddy manager
	cm := caddy.NewManager(cfg.Caddy.ConfigDir, cfg.Caddy.Enabled)
	if cfg.Caddy.Enabled {
		if cm.IsRunning() {
			log.Printf("caddy: admin API responding")
		} else {
			log.Printf("caddy: admin API not responding — site configs will be written but reload may fail")
		}
	} else {
		log.Printf("caddy: disabled (dev mode — apps accessible via localhost:{port})")
	}

	// Build the deploy pipeline
	pipeline := deploy.NewPipeline(cfg, store, ports, cm)
	pipeline.AddStep(&deploy.CreateDirStep{})
	pipeline.AddStep(&deploy.GitCloneStep{})
	pipeline.AddStep(&deploy.DetectConfigStep{})
	pipeline.AddStep(&deploy.BranchFilterStep{})
	pipeline.AddStep(&deploy.RunHooksStep{Phase: deploy.HookAfterClone})
	pipeline.AddStep(&deploy.SystemPackagesStep{})
	pipeline.AddStep(&deploy.WriteInitializerStep{})
	pipeline.AddStep(&deploy.InstallRubyStep{})
	pipeline.AddStep(&deploy.BundlePlatformStep{})
	pipeline.AddStep(&deploy.InstallGemsStep{})
	pipeline.AddStep(&deploy.InstallNodeStep{})
	pipeline.AddStep(&deploy.DetectJSPMStep{})
	pipeline.AddStep(&deploy.InstallJSDepsStep{})
	pipeline.AddStep(&deploy.RunHooksStep{Phase: deploy.HookBeforeBuild})
	pipeline.AddStep(&deploy.SetupDatabaseStep{})
	pipeline.AddStep(&deploy.WriteEnvStep{})
	pipeline.AddStep(&deploy.RunHooksStep{Phase: deploy.HookBeforeMigrate})
	pipeline.AddStep(&deploy.DBPrepareStep{})
	pipeline.AddStep(&deploy.AssetPrecompileStep{})
	pipeline.AddStep(&deploy.SeedStep{})
	pipeline.AddStep(&deploy.RunHooksStep{Phase: deploy.HookAfterBuild})
	pipeline.AddStep(&deploy.AllocatePortStep{})
	pipeline.AddStep(&deploy.StartProcessesStep{})
	pipeline.AddStep(&deploy.HealthCheckStep{})
	pipeline.AddStep(&deploy.CaddyConfigStep{})
	pipeline.AddStep(&deploy.RunHooksStep{Phase: deploy.HookAfterDeploy})
	pipeline.AddStep(&deploy.CallbackStep{})

	srv := server.New(cfg, store, ports, queue, cm)
	srv.SetDeployFunc(func(ctx context.Context, state *app.AppState, redeploy bool) error {
		return pipeline.Run(ctx, state, redeploy)
	})

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	queue.Start(ctx)

	// Start heartbeat
	hb := heartbeat.New(cfg, store)
	hb.Start(30 * time.Second)

	// Start process crash monitor
	mon := monitor.New(cfg, store, 15*time.Second)
	mon.Start()

	go func() {
		if err := srv.Start(); err != nil && err.Error() != "http: Server closed" {
			log.Fatalf("server: %v", err)
		}
	}()

	log.Printf("rad %s started (pid=%d)", version.Version, os.Getpid())

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}

	queue.Stop()
	hb.Stop()
	mon.Stop()
	log.Println("rad stopped")
}

func runUpdate(args []string) {
	fs := flag.NewFlagSet("update", flag.ExitOnError)
	check := fs.Bool("check", false, "only check for updates, do not install")
	force := fs.Bool("force", false, "skip confirmation prompt")
	fs.BoolVar(force, "f", false, "skip confirmation prompt")
	fs.Parse(args)

	fmt.Printf("rad %s — checking for updates...\n", version.Version)

	info, err := updater.CheckForUpdate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if info == nil {
		fmt.Println("Already up to date.")
		return
	}

	fmt.Printf("Update available: %s → %s\n", version.Version, info.Version)
	if info.Changelog != "" {
		fmt.Printf("\nChangelog:\n%s\n\n", info.Changelog)
	}

	if *check {
		return
	}

	if !*force {
		fmt.Printf("Install update? [y/N] ")
		var answer string
		fmt.Scanln(&answer)
		if answer != "y" && answer != "Y" {
			fmt.Println("Update cancelled.")
			return
		}
	}

	if err := updater.Apply(info); err != nil {
		fmt.Fprintf(os.Stderr, "Update failed: %v\n", err)
		os.Exit(1)
	}

	if updater.IsSystemd() {
		fmt.Println("Restarting rad service...")
		restart := exec.Command("systemctl", "restart", "rad")
		restart.Stdout = os.Stdout
		restart.Stderr = os.Stderr
		if err := restart.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to restart: %v\n", err)
			fmt.Println("Restart manually with: sudo systemctl restart rad")
		}
	}
}
