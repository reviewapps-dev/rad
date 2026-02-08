package process

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"
)

type Info struct {
	Cmd *exec.Cmd
	PID int
}

func Start(cmd *exec.Cmd) (*Info, error) {
	// Use process group so we can kill the whole tree
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}

	return &Info{
		Cmd: cmd,
		PID: cmd.Process.Pid,
	}, nil
}

func Stop(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return nil
	}

	// Try SIGTERM to the process group
	pgid, err := syscall.Getpgid(pid)
	if err == nil {
		syscall.Kill(-pgid, syscall.SIGTERM)
	} else {
		proc.Signal(syscall.SIGTERM)
	}

	// Wait up to 10 seconds for graceful shutdown
	done := make(chan struct{})
	go func() {
		proc.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(10 * time.Second):
		// Force kill the process group
		if pgid, err := syscall.Getpgid(pid); err == nil {
			syscall.Kill(-pgid, syscall.SIGKILL)
		} else {
			proc.Kill()
		}
		return nil
	}
}
