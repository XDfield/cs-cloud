//go:build !windows

package app

import (
	"os"
	"syscall"
	"time"
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
		a.RemoveAgentPID()
		return false
	}

	if agentPID, err := a.ReadAgentPID(); err == nil && agentPID > 0 {
		forceKill(agentPID)
		a.RemoveAgentPID()
	}

	proc, _ := os.FindProcess(-pid)
	if proc != nil {
		_ = proc.Signal(os.Interrupt)
	} else {
		proc, _ = os.FindProcess(pid)
		_ = proc.Signal(os.Interrupt)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !a.IsProcessRunning(pid) {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	if a.IsProcessRunning(pid) {
		forceKill(pid)
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
