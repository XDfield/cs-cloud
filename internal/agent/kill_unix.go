//go:build !windows

package agent

import (
	"os"
	"os/exec"
	"syscall"
)

func SetCmdProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func SignalTerminate(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGTERM)
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = p.Signal(syscall.SIGTERM)
}

func KillProcessTree(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	p, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = p.Signal(syscall.SIGKILL)
}
