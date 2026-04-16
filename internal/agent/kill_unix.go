//go:build !windows

package agent

import (
	"os"
	"os/exec"
	"syscall"
)

func setCmdProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func signalTerminate(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = p.Signal(syscall.SIGTERM)
}

func killProcessTree(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = p.Signal(syscall.SIGKILL)
}
