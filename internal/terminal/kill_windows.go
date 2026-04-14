//go:build windows

package terminal

import (
	"os/exec"
	"strconv"
)

func killProcessTree(pid int) {
	cmd := exec.Command("taskkill", "/pid", strconv.Itoa(pid), "/f", "/t")
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run()
}
