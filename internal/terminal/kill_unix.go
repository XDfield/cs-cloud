//go:build !windows

package terminal

import (
	"os"
	"syscall"
)

func killProcessTree(pid int) {
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = p.Signal(syscall.SIGTERM)
}
