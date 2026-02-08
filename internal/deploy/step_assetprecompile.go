package deploy

import (
	"fmt"
	"os"

	"github.com/reviewapps-dev/rad/internal/rv"
)

type AssetPrecompileStep struct{}

func (s *AssetPrecompileStep) Name() string { return "asset-precompile" }

func (s *AssetPrecompileStep) Run(ctx *StepContext) error {
	buildCmd := ctx.AppState.BuildCommand
	if buildCmd == "" {
		buildCmd = "bin/rails assets:precompile"
	}

	ctx.Logger.Log("running: %s", buildCmd)

	cmd := rv.ExecInDir(ctx.RepoDir, ctx.AppState.RubyVersion, buildEnvSlice(ctx.EnvMap), buildCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("asset precompile: %w", err)
	}

	ctx.Logger.Log("assets precompiled")
	return nil
}
