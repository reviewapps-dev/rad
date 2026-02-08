package deploy

import (
	"path/filepath"

	"github.com/reviewapps-dev/rad/internal/env"
)

type WriteEnvStep struct{}

func (s *WriteEnvStep) Name() string { return "write-env" }

func (s *WriteEnvStep) Run(ctx *StepContext) error {
	subdomain := ctx.AppState.Subdomain
	if subdomain == "" {
		subdomain = ctx.AppState.AppID
	}

	// Build the full env map
	fullEnv := env.Build(ctx.AppState.AppID, subdomain, ctx.AppState.EnvVars, ctx.EnvMap, env.WithDevMode(ctx.Config.Dev))

	// Merge back into ctx.EnvMap so downstream steps use it
	for k, v := range fullEnv {
		ctx.EnvMap[k] = v
	}

	// Write .env file
	envPath := filepath.Join(ctx.AppDir, ".env")
	ctx.Logger.Log("writing .env to %s (%d vars)", envPath, len(ctx.EnvMap))

	return env.WriteFile(envPath, ctx.EnvMap)
}
