package deploy

import (
	"fmt"
	"os"

	"github.com/reviewapps-dev/rad/internal/reviewappsyml"
	"github.com/reviewapps-dev/rad/internal/rv"
)

// HookPhase identifies when hooks run in the pipeline.
type HookPhase string

const (
	HookAfterClone     HookPhase = "after_clone"
	HookBeforeBuild    HookPhase = "before_build"
	HookAfterBuild     HookPhase = "after_build"
	HookBeforeMigrate  HookPhase = "before_migrate"
	HookAfterDeploy    HookPhase = "after_deploy"
	HookBeforeTeardown HookPhase = "before_teardown"
	HookOnFailure      HookPhase = "on_failure"
)

// RunHooksStep runs the hook commands for a given phase.
// Hooks come from reviewapps.yml and execute in the app's directory
// with the app's Ruby version and environment.
type RunHooksStep struct {
	Phase HookPhase
}

func (s *RunHooksStep) Name() string { return "hooks-" + string(s.Phase) }

func (s *RunHooksStep) Run(ctx *StepContext) error {
	hooks := s.getHooks(ctx)
	if len(hooks) == 0 {
		return nil
	}

	ctx.Logger.Log("running %d %s hook(s)", len(hooks), s.Phase)

	for i, hook := range hooks {
		ctx.Logger.Log("  [%d/%d] %s", i+1, len(hooks), hook)

		cmd := rv.ExecInDir(ctx.RepoDir, ctx.AppState.RubyVersion, buildEnvSlice(ctx.EnvMap), hook)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("hook %q failed: %w", hook, err)
		}
	}

	ctx.Logger.Log("%s hooks complete", s.Phase)
	return nil
}

func (s *RunHooksStep) getHooks(ctx *StepContext) []string {
	if ctx.ReviewConfig == nil {
		return nil
	}
	return hooksForPhase(ctx.ReviewConfig, s.Phase)
}

// hooksForPhase returns the hook commands for a given phase from a parsed config.
// Exported so the teardown handler and pipeline error path can also use it.
func hooksForPhase(cfg *reviewappsyml.Config, phase HookPhase) []string {
	switch phase {
	case HookAfterClone:
		return cfg.Hooks.AfterClone
	case HookBeforeBuild:
		return cfg.Hooks.BeforeBuild
	case HookAfterBuild:
		return cfg.Hooks.AfterBuild
	case HookBeforeMigrate:
		return cfg.Hooks.BeforeMigrate
	case HookAfterDeploy:
		return cfg.Hooks.AfterDeploy
	case HookBeforeTeardown:
		return cfg.Hooks.BeforeTeardown
	case HookOnFailure:
		return cfg.Hooks.OnFailure
	default:
		return nil
	}
}

// RunHooksFromConfig runs hooks for a phase outside of the pipeline (e.g. teardown, failure).
// This is a standalone function that doesn't need a StepContext.
func RunHooksFromConfig(cfg *reviewappsyml.Config, phase HookPhase, repoDir, rubyVersion string, env []string) error {
	hooks := hooksForPhase(cfg, phase)
	if len(hooks) == 0 {
		return nil
	}

	for _, hook := range hooks {
		cmd := rv.ExecInDir(repoDir, rubyVersion, env, hook)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("hook %q failed: %w", hook, err)
		}
	}
	return nil
}
