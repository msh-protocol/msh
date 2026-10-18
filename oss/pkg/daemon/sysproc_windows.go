//go:build windows

package daemon

import (
	"os/exec"
	"syscall"
)

func setDetached(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: 0x08000000, // DETACHED_PROCESS
	}
}
