package deploy

import (
	"os"
	"path/filepath"
)

type CreateDirStep struct{}

func (s *CreateDirStep) Name() string { return "create-dir" }

func (s *CreateDirStep) Run(ctx *StepContext) error {
	appDir := filepath.Join(ctx.Config.Paths.AppsDir, ctx.AppState.AppID)
	ctx.AppDir = appDir
	ctx.RepoDir = filepath.Join(appDir, "repo")
	ctx.AppState.AppDir = appDir

	if ctx.Redeploy {
		ctx.Logger.Log("redeploy: reusing app directory %s", appDir)
		return nil
	}

	ctx.Logger.Log("creating app directory: %s", appDir)

	// Clean up any existing directory for fresh deploy
	if _, err := os.Stat(appDir); err == nil {
		ctx.Logger.Log("removing existing directory")
		if err := os.RemoveAll(appDir); err != nil {
			return err
		}
	}

	return os.MkdirAll(ctx.RepoDir, 0755)
}
