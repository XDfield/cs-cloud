//go:build !windows

package app

import (
	"os/exec"
	"syscall"
)

func newDaemonCmd(exe string, args []string) *exec.Cmd {
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	return cmd
}
