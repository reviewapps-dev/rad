package deploy

import (
	"github.com/reviewapps-dev/rad/internal/caddy"
)

type CaddyConfigStep struct{}

func (s *CaddyConfigStep) Name() string { return "caddy-config" }

func (s *CaddyConfigStep) Run(ctx *StepContext) error {
	if ctx.Caddy == nil || !ctx.Caddy.Enabled {
		ctx.Logger.Log("caddy not enabled, skipping site config")
		return nil
	}

	subdomain := ctx.AppState.Subdomain
	if subdomain == "" {
		subdomain = ctx.AppState.AppID
	}

	cfg := caddy.SiteConfig{
		AppID:     ctx.AppState.AppID,
		Subdomain: subdomain,
		Port:      ctx.Port,
		LogDir:    ctx.Config.Paths.LogDir,
	}

	if err := ctx.Caddy.WriteSiteConfig(cfg); err != nil {
		return err
	}

	ctx.Logger.Log("wrote caddy config for %s.srv.reviewapps.dev → localhost:%d", subdomain, ctx.Port)

	// Reload Caddy to pick up the new config
	if err := ctx.Caddy.Reload(); err != nil {
		ctx.Logger.Log("caddy reload failed (non-fatal): %v", err)
		// Non-fatal — the app is still accessible via localhost:{port}
	} else {
		ctx.Logger.Log("caddy reloaded")
	}

	return nil
}
