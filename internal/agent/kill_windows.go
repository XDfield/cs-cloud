//go:build windows

package agent

import (
	"fmt"
	"os"
	"os/exec"
)

func setCmdProcessGroup(cmd *exec.Cmd) {
}

func signalTerminate(pid int) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = proc.Signal(os.Interrupt)
}

func killProcessTree(pid int) {
	cmd := exec.Command("taskkill", "/pid", fmt.Sprintf("%d", pid), "/f", "/t")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run()
}
