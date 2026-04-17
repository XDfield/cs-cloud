//go:build !windows

package cli

import (
	"os/exec"
	"syscall"
)

func newDaemonCmd(exe string, args []string) *exec.Cmd {
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}
