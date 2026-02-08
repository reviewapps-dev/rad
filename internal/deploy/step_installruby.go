package deploy

import (
	"strings"

	"github.com/reviewapps-dev/rad/internal/app"
	"github.com/reviewapps-dev/rad/internal/rv"
)

type InstallRubyStep struct{}

func (s *InstallRubyStep) Name() string { return "install-ruby" }

func (s *InstallRubyStep) Run(ctx *StepContext) error {
	_ = ctx.Store.UpdateStatus(ctx.AppState.AppID, app.StatusBuilding, "")

	version := strings.TrimSpace(ctx.AppState.RubyVersion)
	if version == "" {
		version = ctx.Config.Defaults.RubyVersion
		ctx.AppState.RubyVersion = version
	}

	ctx.Logger.Log("installing ruby %s via rv", version)

	if err := rv.Install(version); err != nil {
		return err
	}

	rubyPath, err := rv.FindRuby(version)
	if err != nil {
		ctx.Logger.Log("could not determine ruby path: %v (continuing)", err)
	} else {
		ctx.RubyPath = rubyPath
		ctx.Logger.Log("ruby path: %s", rubyPath)
	}

	return nil
}
