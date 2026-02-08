package deploy

import "fmt"

type AllocatePortStep struct{}

func (s *AllocatePortStep) Name() string { return "allocate-port" }

func (s *AllocatePortStep) Run(ctx *StepContext) error {
	// On redeploy, reuse the existing port if we have one
	if ctx.Redeploy {
		if existingPort, ok := ctx.Ports.GetPort(ctx.AppState.AppID); ok {
			ctx.Port = existingPort
			ctx.AppState.Port = existingPort
			ctx.EnvMap["PORT"] = fmt.Sprintf("%d", existingPort)
			ctx.Logger.Log("redeploy: reusing port %d", existingPort)
			return nil
		}
	}

	port, err := ctx.Ports.Allocate(ctx.AppState.AppID)
	if err != nil {
		return fmt.Errorf("port allocation: %w", err)
	}

	ctx.Port = port
	ctx.AppState.Port = port
	_ = ctx.Store.SetPort(ctx.AppState.AppID, port)
	ctx.EnvMap["PORT"] = fmt.Sprintf("%d", port)

	ctx.Logger.Log("allocated port %d", port)
	return nil
}
