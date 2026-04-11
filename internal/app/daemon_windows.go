//go:build windows

package app

import (
	"fmt"
	"os"
	"syscall"
	"time"
)

func (a *App) IsProcessRunning(pid int) bool {
	const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
	handle, err := syscall.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false
	}
	syscall.CloseHandle(handle)
	return true
}

func (a *App) StopDaemon() bool {
	pid, err := a.ReadPID()
	if err != nil || !a.IsProcessRunning(pid) {
		a.RemovePID()
		return false
	}

	os.WriteFile(a.stopFile(), []byte(fmt.Sprintf("%d", time.Now().UnixMilli())), 0o644)
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
