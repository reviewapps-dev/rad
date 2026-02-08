package deploy

import (
	"fmt"
	"os"

	"github.com/reviewapps-dev/rad/internal/rv"
)

type SeedStep struct{}

func (s *SeedStep) Name() string { return "seed" }

func (s *SeedStep) Run(ctx *StepContext) error {
	if ctx.Redeploy {
		ctx.Logger.Log("redeploy: skipping seed")
		return nil
	}

	seedCmd := ctx.AppState.SeedCommand
	if seedCmd == "" {
		ctx.Logger.Log("no seed command specified, skipping")
		return nil
	}

	ctx.Logger.Log("running seed: %s", seedCmd)

	cmd := rv.ExecInDir(ctx.RepoDir, ctx.AppState.RubyVersion, buildEnvSlice(ctx.EnvMap), seedCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("seed: %w", err)
	}

	ctx.Logger.Log("seed complete")
	return nil
}
