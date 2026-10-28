//go:build windows

package daemon

import (
	"os/exec"
	"syscall"
)

const (
	stillActive             = 259
	processQueryLimitedInfo = 0x1000
	processQueryInfo        = 0x0400
)

func setDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000, // DETACHED_PROCESS
	}
}

// checkProcessLiveness checks whether the process is alive on Windows.
// Returns (alive, exitCode).
func checkProcessLiveness(pid int) (bool, int) {
	if pid <= 0 {
		return false, -1
	}

	// Try PROCESS_QUERY_LIMITED_INFORMATION first, fall back to PROCESS_QUERY_INFORMATION
	handle, err := syscall.OpenProcess(processQueryLimitedInfo, false, uint32(pid))
	if err != nil {
		handle, err = syscall.OpenProcess(processQueryInfo, false, uint32(pid))
		if err != nil {
			return false, -1
		}
	}
	defer syscall.CloseHandle(handle)

	var exitCode uint32
	if err := syscall.GetExitCodeProcess(handle, &exitCode); err != nil {
		return false, -1
	}

	if exitCode == stillActive {
		return true, 0
	}
	return false, int(exitCode)
}
