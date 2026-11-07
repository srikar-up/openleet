//go:build !windows

package runner

import (
	"os/exec"
	"syscall"
)

// setProcessGroup configures process group isolation on POSIX systems.
func setProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// terminateProcessTree sends SIGKILL to the entire process group on POSIX.
func terminateProcessTree(pid int) error {
	return syscall.Kill(-pid, syscall.SIGKILL)
}
