//go:build windows

package agent

import (
	"fmt"
	"os"
	"os/exec"
)

func SetCmdProcessGroup(cmd *exec.Cmd) {
}

func SignalTerminate(pid int) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = proc.Signal(os.Interrupt)
}

func KillProcessTree(pid int) {
	cmd := exec.Command("taskkill", "/pid", fmt.Sprintf("%d", pid), "/f", "/t")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run()
}
