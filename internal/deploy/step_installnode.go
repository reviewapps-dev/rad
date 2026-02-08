package deploy

import (
	"strings"

	"github.com/reviewapps-dev/rad/internal/fnm"
)

type InstallNodeStep struct{}

func (s *InstallNodeStep) Name() string { return "install-node" }

func (s *InstallNodeStep) Run(ctx *StepContext) error {
	version := strings.TrimSpace(ctx.AppState.NodeVersion)
	if version == "" {
		ctx.Logger.Log("no node version specified, skipping")
		return nil
	}

	ctx.Logger.Log("installing node %s via fnm", version)

	if err := fnm.Install(version); err != nil {
		return err
	}

	nodeBinDir, err := fnm.Which(version)
	if err != nil {
		ctx.Logger.Log("could not determine node path: %v (continuing)", err)
	} else {
		ctx.NodePath = nodeBinDir
		ctx.Logger.Log("node bin dir: %s", nodeBinDir)
	}

	return nil
}
