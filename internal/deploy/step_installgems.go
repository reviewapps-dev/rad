package deploy

import (
	"github.com/reviewapps-dev/rad/internal/rv"
)

type InstallGemsStep struct{}

func (s *InstallGemsStep) Name() string { return "install-gems" }

func (s *InstallGemsStep) Run(ctx *StepContext) error {
	ctx.Logger.Log("installing gems via rv clean-install")

	env := buildEnvSlice(ctx.EnvMap)
	if err := rv.CleanInstall(ctx.RepoDir, env); err != nil {
		return err
	}

	ctx.Logger.Log("gems installed")
	return nil
}

func buildEnvSlice(m map[string]string) []string {
	result := make([]string, 0, len(m))
	for k, v := range m {
		result = append(result, k+"="+v)
	}
	return result
}
