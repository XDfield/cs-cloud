//go:build !windows

package terminal

import (
	"os"
	"syscall"
)

func killProcessTree(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = p.Signal(syscall.SIGTERM)
}
