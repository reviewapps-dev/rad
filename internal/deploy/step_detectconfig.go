package deploy

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/reviewapps-dev/rad/internal/app"
	"github.com/reviewapps-dev/rad/internal/reviewappsyml"
)

type DetectConfigStep struct{}

func (s *DetectConfigStep) Name() string { return "detect-config" }

func (s *DetectConfigStep) Run(ctx *StepContext) error {
	ymlPath := filepath.Join(ctx.RepoDir, "reviewapps.yml")

	if _, err := os.Stat(ymlPath); err == nil {
		ctx.Logger.Log("found reviewapps.yml")
		cfg, err := reviewappsyml.Parse(ymlPath)
		if err != nil {
			return err
		}
		ctx.ReviewConfig = cfg

		// Override app state from yml if not set in deploy request
		if ctx.AppState.RubyVersion == "" && cfg.Ruby != "" {
			ctx.AppState.RubyVersion = cfg.Ruby
		}
		if ctx.AppState.NodeVersion == "" && cfg.Node != "" {
			ctx.AppState.NodeVersion = cfg.Node
		}
		if ctx.AppState.DatabaseAdapter == "" && cfg.Database != "" {
			ctx.AppState.DatabaseAdapter = cfg.Database
		}
		if len(ctx.AppState.Databases) == 0 && len(cfg.Databases) > 0 {
			ctx.AppState.Databases = cfg.Databases
		}
		if ctx.AppState.BuildCommand == "" && cfg.Build.Command != "" {
			ctx.AppState.BuildCommand = cfg.Build.Command
		}
		if ctx.AppState.SeedCommand == "" && cfg.Seed.Command != "" {
			ctx.AppState.SeedCommand = cfg.Seed.Command
		}

		// Merge env vars (yml env is lower priority than request env)
		if cfg.Env != nil {
			for k, v := range cfg.Env {
				if _, exists := ctx.AppState.EnvVars[k]; !exists {
					if ctx.AppState.EnvVars == nil {
						ctx.AppState.EnvVars = make(map[string]string)
					}
					ctx.AppState.EnvVars[k] = v
				}
			}
		}

		// Apply app_path for monorepo support
		if cfg.AppPath != "" && cfg.AppPath != "." {
			ctx.RepoDir = filepath.Join(ctx.RepoDir, cfg.AppPath)
			ctx.Logger.Log("monorepo: app_path=%s, working dir=%s", cfg.AppPath, ctx.RepoDir)
			if _, err := os.Stat(ctx.RepoDir); err != nil {
				return fmt.Errorf("app_path %q does not exist in repo", cfg.AppPath)
			}
		}

		// Load process declarations from reviewapps.yml
		if len(cfg.Processes) > 0 {
			ctx.Processes = cfg.Processes
			names := make([]string, 0, len(cfg.Processes))
			for name := range cfg.Processes {
				names = append(names, name)
			}
			ctx.Logger.Log("processes from reviewapps.yml: %v", names)
		}
	} else {
		ctx.Logger.Log("no reviewapps.yml found, using defaults")
	}

	// Merge request-level hooks into ReviewConfig (yml hooks take priority)
	if ctx.AppState.Hooks != nil {
		if ctx.ReviewConfig == nil {
			ctx.ReviewConfig = &reviewappsyml.Config{}
		}
		mergeHooks(&ctx.ReviewConfig.Hooks, ctx.AppState.Hooks)
	}

	// Detect ruby version from .ruby-version if still not set
	if ctx.AppState.RubyVersion == "" {
		rvFile := filepath.Join(ctx.RepoDir, ".ruby-version")
		if data, err := os.ReadFile(rvFile); err == nil {
			ctx.AppState.RubyVersion = string(data)
			ctx.Logger.Log("detected ruby version from .ruby-version: %s", ctx.AppState.RubyVersion)
		}
	}

	// Detect node version from .node-version or .nvmrc
	if ctx.AppState.NodeVersion == "" {
		for _, f := range []string{".node-version", ".nvmrc"} {
			path := filepath.Join(ctx.RepoDir, f)
			if data, err := os.ReadFile(path); err == nil {
				ctx.AppState.NodeVersion = string(data)
				ctx.Logger.Log("detected node version from %s: %s", f, ctx.AppState.NodeVersion)
				break
			}
		}
	}

	return nil
}

// mergeHooks merges request-level hooks into yml hooks.
// yml hooks take priority — request hooks only fill in empty phases.
func mergeHooks(dst *struct {
	AfterClone     []string `yaml:"after_clone"`
	BeforeBuild    []string `yaml:"before_build"`
	AfterBuild     []string `yaml:"after_build"`
	BeforeMigrate  []string `yaml:"before_migrate"`
	AfterDeploy    []string `yaml:"after_deploy"`
	BeforeTeardown []string `yaml:"before_teardown"`
	OnFailure      []string `yaml:"on_failure"`
}, src *app.Hooks) {
	if len(dst.AfterClone) == 0 {
		dst.AfterClone = src.AfterClone
	}
	if len(dst.BeforeBuild) == 0 {
		dst.BeforeBuild = src.BeforeBuild
	}
	if len(dst.AfterBuild) == 0 {
		dst.AfterBuild = src.AfterBuild
	}
	if len(dst.BeforeMigrate) == 0 {
		dst.BeforeMigrate = src.BeforeMigrate
	}
	if len(dst.AfterDeploy) == 0 {
		dst.AfterDeploy = src.AfterDeploy
	}
	if len(dst.BeforeTeardown) == 0 {
		dst.BeforeTeardown = src.BeforeTeardown
	}
	if len(dst.OnFailure) == 0 {
		dst.OnFailure = src.OnFailure
	}
}
