package deploy

import (
	"github.com/reviewapps-dev/rad/internal/app"
	"github.com/reviewapps-dev/rad/internal/git"
)

type GitCloneStep struct{}

func (s *GitCloneStep) Name() string { return "git-clone" }

func (s *GitCloneStep) Run(ctx *StepContext) error {
	_ = ctx.Store.UpdateStatus(ctx.AppState.AppID, app.StatusCloning, "")

	if ctx.Redeploy {
		ctx.Logger.Log("fetching updates for %s (branch: %s)", ctx.AppState.RepoURL, ctx.AppState.Branch)
		if err := git.FetchAndReset(ctx.RepoDir, ctx.AppState.Branch); err != nil {
			return err
		}
	} else {
		ctx.Logger.Log("cloning %s (branch: %s) into %s", ctx.AppState.RepoURL, ctx.AppState.Branch, ctx.RepoDir)
		if err := git.Clone(ctx.AppState.RepoURL, ctx.AppState.Branch, ctx.RepoDir); err != nil {
			return err
		}
	}

	// Get actual commit SHA
	sha, err := git.GetCommitSHA(ctx.RepoDir)
	if err == nil {
		ctx.AppState.CommitSHA = sha
		ctx.Logger.Log("commit: %s", sha)
	}

	// Init submodules (best-effort)
	if err := git.InitSubmodules(ctx.RepoDir); err != nil {
		ctx.Logger.Log("submodule init: %v (continuing)", err)
	}

	return nil
}
