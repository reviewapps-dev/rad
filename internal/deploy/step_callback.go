package deploy

import (
	"fmt"

	"github.com/reviewapps-dev/rad/internal/callback"
)

type CallbackStep struct{}

func (s *CallbackStep) Name() string { return "callback" }

func (s *CallbackStep) Run(ctx *StepContext) error {
	if ctx.AppState.CallbackURL == "" {
		ctx.Logger.Log("no callback URL, skipping")
		return nil
	}

	client := callback.NewClient(ctx.Config.API.APIKey)

	url := fmt.Sprintf("http://localhost:%d", ctx.Port)
	if ctx.AppState.Subdomain != "" {
		url = fmt.Sprintf("https://%s.srv.reviewapps.dev", ctx.AppState.Subdomain)
	}

	payload := callback.StatusPayload{
		AppID:     ctx.AppState.AppID,
		Status:    string(ctx.AppState.Status),
		Port:      ctx.Port,
		URL:       url,
		CommitSHA: ctx.AppState.CommitSHA,
	}

	ctx.Logger.Log("sending callback to %s", ctx.AppState.CallbackURL)
	return client.SendStatus(ctx.AppState.CallbackURL, payload)
}
