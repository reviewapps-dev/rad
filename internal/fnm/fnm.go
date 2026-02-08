package fnm

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Install(nodeVersion string) error {
	cmd := exec.Command("fnm", "install", nodeVersion)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("fnm install %s: %w", nodeVersion, err)
	}
	return nil
}

func Which(nodeVersion string) (string, error) {
	// fnm env --use-on-cd gives us the path; we can also use fnm exec
	cmd := exec.Command("fnm", "exec", "--using", nodeVersion, "--", "which", "node")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("fnm which node %s: %w", nodeVersion, err)
	}
	nodePath := strings.TrimSpace(string(out))
	return filepath.Dir(nodePath), nil // return the directory containing node
}

// ListInstalled returns the installed Node versions.
func ListInstalled() []string {
	out, err := exec.Command("fnm", "list").Output()
	if err != nil {
		return nil
	}
	var versions []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == "system" || strings.Contains(line, "system") {
			continue
		}
		// Format: "* v22.1.0 default" or "v20.10.0"
		for _, field := range strings.Fields(line) {
			if strings.HasPrefix(field, "v") {
				versions = append(versions, strings.TrimPrefix(field, "v"))
				break
			}
		}
	}
	return versions
}

// ExecInDir returns a command that runs under the specified node version.
func ExecInDir(dir string, nodeVersion string, env []string, args ...string) *exec.Cmd {
	cmdArgs := []string{"exec", "--using", nodeVersion, "--"}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command("fnm", cmdArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	return cmd
}
