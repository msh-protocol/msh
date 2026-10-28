//go:build !windows

package daemon

import (
	"os"
	"os/exec"
	"syscall"
)

func setDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}
}

// checkProcessLiveness checks whether the process is alive on Unix.
// Returns (alive, exitCode).
func checkProcessLiveness(pid int) (bool, int) {
	if pid <= 0 {
		return false, -1
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, -1
	}
	// Signal 0 tests whether the process exists and can receive signals
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return false, -1
	}
	return true, 0
}
