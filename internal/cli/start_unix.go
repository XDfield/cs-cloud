//go:build !windows

package cli

import "os/exec"

func newDaemonCmd(exe string, args []string) *exec.Cmd {
	return exec.Command(exe, args...)
}
