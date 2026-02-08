package deploy

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
)

var validPkgName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9.+\-:]+$`)

type SystemPackagesStep struct{}

func (s *SystemPackagesStep) Name() string { return "system-packages" }

func (s *SystemPackagesStep) Run(ctx *StepContext) error {
	if ctx.ReviewConfig == nil || len(ctx.ReviewConfig.SystemPackages) == 0 {
		return nil
	}

	pkgs := ctx.ReviewConfig.SystemPackages

	// Validate package names to prevent injection
	for _, pkg := range pkgs {
		if !validPkgName.MatchString(pkg) {
			return fmt.Errorf("invalid package name %q — must be alphanumeric with hyphens/dots/colons only", pkg)
		}
	}

	if ctx.Config.Dev {
		ctx.Logger.Log("system_packages: skipping in dev mode (ensure these are installed: %s)", strings.Join(pkgs, ", "))
		return nil
	}

	ctx.Logger.Log("installing system packages: %s", strings.Join(pkgs, ", "))

	args := append([]string{"apt-get", "install", "-y"}, pkgs...)
	cmd := exec.Command("sudo", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		ctx.Logger.Log("apt-get output: %s", string(out))
		return fmt.Errorf("system_packages install failed: %w", err)
	}

	ctx.Logger.Log("system packages installed successfully")
	return nil
}
