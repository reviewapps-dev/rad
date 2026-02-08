package deploy

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/reviewapps-dev/rad/internal/rv"
)

type BundlePlatformStep struct{}

func (s *BundlePlatformStep) Name() string { return "bundle-platform" }

func (s *BundlePlatformStep) Run(ctx *StepContext) error {
	platform := detectPlatform()
	ctx.Logger.Log("ensuring platform %s is in Gemfile.lock", platform)

	// Use -S so ruby searches PATH for the bundle script
	cmd := rv.RunInDir(ctx.RepoDir, ctx.AppState.RubyVersion, nil,
		"-S", "bundle", "lock", "--add-platform", platform)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bundle lock --add-platform %s: %w", platform, err)
	}

	return nil
}

func detectPlatform() string {
	// Use uname -m to get the real hardware arch (not Rosetta-translated)
	if out, err := exec.Command("uname", "-m").Output(); err == nil {
		arch := strings.TrimSpace(string(out))
		switch arch {
		case "arm64":
			arch = "aarch64"
		}
		return fmt.Sprintf("%s-%s", arch, runtime.GOOS)
	}

	// Fallback to Go runtime
	arch := runtime.GOARCH
	switch arch {
	case "arm64":
		arch = "aarch64"
	case "amd64":
		arch = "x86_64"
	}
	return fmt.Sprintf("%s-%s", arch, runtime.GOOS)
}
