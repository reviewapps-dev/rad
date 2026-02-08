package git

import (
	"fmt"
	"os/exec"
)

func Clone(repoURL, branch, dest string) error {
	args := []string{"clone", "--depth", "1", "--branch", branch, repoURL, dest}
	cmd := exec.Command("git", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone: %w\n%s", err, string(out))
	}
	return nil
}

func InitSubmodules(repoDir string) error {
	cmd := exec.Command("git", "submodule", "update", "--init", "--recursive")
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git submodule: %w\n%s", err, string(out))
	}
	return nil
}

func FetchAndReset(repoDir, branch string) error {
	// Fetch latest from origin
	fetch := exec.Command("git", "fetch", "origin", branch)
	fetch.Dir = repoDir
	if out, err := fetch.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch: %w\n%s", err, string(out))
	}

	// Reset to latest
	reset := exec.Command("git", "reset", "--hard", "origin/"+branch)
	reset.Dir = repoDir
	if out, err := reset.CombinedOutput(); err != nil {
		return fmt.Errorf("git reset: %w\n%s", err, string(out))
	}

	return nil
}

func GetCommitSHA(repoDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return string(out[:len(out)-1]), nil // trim newline
}
