package rv

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const DefaultBin = "/opt/homebrew/bin/rv"

func Install(rubyVersion string) error {
	cmd := exec.Command(DefaultBin, "ruby", "install", rubyVersion)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rv ruby install %s: %w", rubyVersion, err)
	}
	return nil
}

func FindRuby(rubyVersion string) (string, error) {
	cmd := exec.Command(DefaultBin, "ruby", "find", rubyVersion)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("rv ruby find %s: %w", rubyVersion, err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RunInDir runs a ruby script via rv in the given directory.
// args are passed to "rv ruby run VERSION -- <args>", which invokes "ruby <args>".
// Use for: bin/rails, -S bundle, -e "code"
func RunInDir(dir string, rubyVersion string, env []string, args ...string) *exec.Cmd {
	cmdArgs := []string{"ruby", "run", rubyVersion, "--"}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.Command(DefaultBin, cmdArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	return cmd
}

// ExecInDir runs an arbitrary shell command with rv's ruby on PATH.
// rv ruby run VERSION -- -e "exec 'bash', '-c', 'cmd'" sets up the ruby
// env (PATH, GEM_HOME, etc.) then exec's into bash.
func ExecInDir(dir string, rubyVersion string, env []string, shellCmd string) *exec.Cmd {
	rubyCode := fmt.Sprintf("exec 'bash', '-c', %%q(%s)", shellCmd)
	return RunInDir(dir, rubyVersion, env, "-e", rubyCode)
}

// ListInstalled returns the installed Ruby versions.
func ListInstalled() []string {
	out, err := exec.Command(DefaultBin, "ruby", "list").Output()
	if err != nil {
		return nil
	}
	var versions []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "[installed]") {
			continue
		}
		// Format: "ruby-3.4.1  [installed] /path/to/ruby"
		parts := strings.Fields(line)
		if len(parts) > 0 {
			ver := strings.TrimPrefix(parts[0], "ruby-")
			// Deduplicate (rv can show same version from multiple sources)
			found := false
			for _, v := range versions {
				if v == ver {
					found = true
					break
				}
			}
			if !found {
				versions = append(versions, ver)
			}
		}
	}
	return versions
}

// CleanInstall runs `rv clean-install` in the given directory.
func CleanInstall(dir string, env []string) error {
	cmd := exec.Command(DefaultBin, "clean-install")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rv clean-install: %w", err)
	}
	return nil
}
