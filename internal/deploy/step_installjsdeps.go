package deploy

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/reviewapps-dev/rad/internal/fnm"
)

type InstallJSDepsStep struct{}

func (s *InstallJSDepsStep) Name() string { return "install-js-deps" }

func (s *InstallJSDepsStep) Run(ctx *StepContext) error {
	if ctx.JSPackageManager == "" {
		ctx.Logger.Log("no JS package manager, skipping")
		return nil
	}

	nodeVersion := strings.TrimSpace(ctx.AppState.NodeVersion)
	if nodeVersion == "" {
		ctx.Logger.Log("no node version specified, skipping JS deps install")
		return nil
	}

	ctx.Logger.Log("installing JS deps with %s", ctx.JSPackageManager)

	var cmd *exec.Cmd
	switch ctx.JSPackageManager {
	case "npm":
		cmd = fnm.ExecInDir(ctx.RepoDir, nodeVersion, buildEnvSlice(ctx.EnvMap), "npm", "ci")
	case "yarn":
		cmd = fnm.ExecInDir(ctx.RepoDir, nodeVersion, buildEnvSlice(ctx.EnvMap), "yarn", "install", "--frozen-lockfile")
	case "pnpm":
		cmd = fnm.ExecInDir(ctx.RepoDir, nodeVersion, buildEnvSlice(ctx.EnvMap), "pnpm", "install", "--frozen-lockfile")
	case "bun":
		cmd = fnm.ExecInDir(ctx.RepoDir, nodeVersion, buildEnvSlice(ctx.EnvMap), "bun", "install", "--frozen-lockfile")
	default:
		return fmt.Errorf("unknown JS package manager: %s", ctx.JSPackageManager)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s install: %w", ctx.JSPackageManager, err)
	}

	ctx.Logger.Log("JS deps installed")
	return nil
}
