//go:build windows

package cli

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func newDaemonCmd(exe string, args []string) *exec.Cmd {
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP | createNoWindow,
		HideWindow:    true,
	}
	return cmd
}
