package deploy

import (
	"os"
	"path/filepath"

	"github.com/reviewapps-dev/rad/internal/initializer"
)

type WriteInitializerStep struct{}

func (s *WriteInitializerStep) Name() string { return "write-initializer" }

func (s *WriteInitializerStep) Run(ctx *StepContext) error {
	initDir := filepath.Join(ctx.RepoDir, "config", "initializers")

	// Only write if config/initializers exists (i.e., it's a Rails app)
	if _, err := os.Stat(initDir); os.IsNotExist(err) {
		ctx.Logger.Log("no config/initializers directory, skipping initializer injection")
		return nil
	}

	dest := filepath.Join(initDir, "_reviewapps.rb")
	ctx.Logger.Log("writing initializer: %s", dest)

	return initializer.Write(dest)
}
