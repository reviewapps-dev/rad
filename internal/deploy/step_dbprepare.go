package deploy

import (
	"fmt"
	"os"

	"github.com/reviewapps-dev/rad/internal/rv"
)

type DBPrepareStep struct{}

func (s *DBPrepareStep) Name() string { return "db-prepare" }

func (s *DBPrepareStep) Run(ctx *StepContext) error {
	// If setup.command is configured, run that instead of db:prepare/db:migrate
	if ctx.ReviewConfig != nil && ctx.ReviewConfig.Setup.Command != "" && !ctx.Redeploy {
		setupCmd := ctx.ReviewConfig.Setup.Command
		ctx.Logger.Log("running setup command: %s", setupCmd)

		cmd := rv.ExecInDir(ctx.RepoDir, ctx.AppState.RubyVersion, buildEnvSlice(ctx.EnvMap), setupCmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("setup command %q: %w", setupCmd, err)
		}

		ctx.Logger.Log("setup command complete")
		return nil
	}

	task := "db:prepare"
	if ctx.Redeploy {
		task = "db:migrate"
	}

	ctx.Logger.Log("running %s", task)

	cmd := rv.RunInDir(ctx.RepoDir, ctx.AppState.RubyVersion, buildEnvSlice(ctx.EnvMap),
		"bin/rails", task)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", task, err)
	}

	ctx.Logger.Log("%s complete", task)
	return nil
}
