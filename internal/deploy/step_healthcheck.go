package deploy

import (
	"time"

	"github.com/reviewapps-dev/rad/internal/app"
	"github.com/reviewapps-dev/rad/internal/health"
)

type HealthCheckStep struct{}

func (s *HealthCheckStep) Name() string { return "health-check" }

func (s *HealthCheckStep) Run(ctx *StepContext) error {
	timeout := 30 * time.Second
	interval := 2 * time.Second
	customPath := ""

	if ctx.ReviewConfig != nil {
		if ctx.ReviewConfig.HealthCheck.Timeout > 0 {
			timeout = time.Duration(ctx.ReviewConfig.HealthCheck.Timeout) * time.Second
		}
		if ctx.ReviewConfig.HealthCheck.Interval > 0 {
			interval = time.Duration(ctx.ReviewConfig.HealthCheck.Interval) * time.Second
		}
		if ctx.ReviewConfig.HealthCheck.Path != "" {
			customPath = ctx.ReviewConfig.HealthCheck.Path
		}
	}

	// In dev mode, don't set Host header — just use localhost
	host := ""
	if !ctx.Config.Dev {
		host = ctx.AppState.Subdomain
		if host == "" {
			host = ctx.AppState.AppID
		}
	}

	ctx.Logger.Log("waiting for health check (timeout=%s, interval=%s)", timeout, interval)

	if err := health.Check(ctx.Port, host, timeout, interval, customPath); err != nil {
		return err
	}

	_ = ctx.Store.UpdateStatus(ctx.AppState.AppID, app.StatusRunning, "")
	ctx.Logger.Log("app is healthy and running on port %d", ctx.Port)
	return nil
}
