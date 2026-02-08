package deploy

import (
	"os"
	"path/filepath"
)

type DetectJSPMStep struct{}

func (s *DetectJSPMStep) Name() string { return "detect-jspm" }

func (s *DetectJSPMStep) Run(ctx *StepContext) error {
	// Detect JS package manager from lock files
	lockFiles := map[string]string{
		"bun.lockb":         "bun",
		"bun.lock":          "bun",
		"pnpm-lock.yaml":    "pnpm",
		"yarn.lock":         "yarn",
		"package-lock.json": "npm",
	}

	for file, pm := range lockFiles {
		if _, err := os.Stat(filepath.Join(ctx.RepoDir, file)); err == nil {
			ctx.JSPackageManager = pm
			ctx.Logger.Log("detected JS package manager: %s (from %s)", pm, file)
			return nil
		}
	}

	// Check if package.json exists at all
	if _, err := os.Stat(filepath.Join(ctx.RepoDir, "package.json")); err == nil {
		ctx.JSPackageManager = "npm"
		ctx.Logger.Log("detected JS package manager: npm (default, package.json present)")
	} else {
		ctx.Logger.Log("no JS package manager detected")
	}

	return nil
}
