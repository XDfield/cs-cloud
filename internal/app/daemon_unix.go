//go:build !windows

package app

import (
	"os"
	"syscall"
)

func (a *App) IsProcessRunning(pid int) bool {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func (a *App) StopDaemon() bool {
	pid, err := a.ReadPID()
	if err != nil || !a.IsProcessRunning(pid) {
		a.RemovePID()
		return false
	}

	proc, _ := os.FindProcess(-pid)
	if proc != nil {
		_ = proc.Signal(os.Interrupt)
	} else {
		proc, _ = os.FindProcess(pid)
		_ = proc.Signal(os.Interrupt)
	}

	a.RemovePID()
	a.RemoveStopFile()
	return true
}

func forceKill(pid int) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = proc.Kill()
}
