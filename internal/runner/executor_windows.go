//go:build windows

package runner

import (
	"os/exec"
	"strconv"
	"syscall"
)

// setProcessGroup configures a process group on Windows so process tree termination can clean child processes.
func setProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags = syscall.CREATE_NEW_PROCESS_GROUP
}

// terminateProcessTree terminates the target PID and all its descendants on Windows.
func terminateProcessTree(pid int) error {
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	return cmd.Run()
}
