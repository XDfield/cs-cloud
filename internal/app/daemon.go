package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func (a *App) pidFile() string {
	return filepath.Join(a.rootDir, "cs-cloud.pid")
}

func (a *App) logFile() string {
	return filepath.Join(a.rootDir, "cloud.log")
}

func (a *App) stopFile() string {
	return filepath.Join(a.rootDir, "cloud.stop")
}

func (a *App) stateFile() string {
	return filepath.Join(a.rootDir, "state")
}

func (a *App) serverFile() string {
	return filepath.Join(a.rootDir, "server_url")
}

func (a *App) modeFile() string {
	return filepath.Join(a.rootDir, "mode")
}

func (a *App) ReadPID() (int, error) {
	b, err := os.ReadFile(a.pidFile())
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return 0, err
	}
	return pid, nil
}

func (a *App) WritePID(pid int) error {
	if err := a.EnsureRootDir(); err != nil {
		return err
	}
	return os.WriteFile(a.pidFile(), []byte(strconv.Itoa(pid)), 0o600)
}

func (a *App) RemovePID() {
	os.Remove(a.pidFile())
}

func (a *App) IsProcessRunning(pid int) bool {
	if isWindows() {
		const PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
		handle, err := syscall.OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
		if err != nil {
			return false
		}
		syscall.CloseHandle(handle)
		return true
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func (a *App) DaemonStatus() (bool, int) {
	pid, err := a.ReadPID()
	if err != nil {
		return false, 0
	}
	if !a.IsProcessRunning(pid) {
		a.RemovePID()
		return false, 0
	}
	return true, pid
}

func (a *App) SaveMode(mode string) error {
	if err := a.EnsureRootDir(); err != nil {
		return err
	}
	return os.WriteFile(a.modeFile(), []byte(mode), 0o644)
}

func (a *App) LoadMode() string {
	b, err := os.ReadFile(a.modeFile())
	if err != nil {
		return "cloud"
	}
	return strings.TrimSpace(string(b))
}

func (a *App) StopDaemon() bool {
	pid, err := a.ReadPID()
	if err != nil || !a.IsProcessRunning(pid) {
		a.RemovePID()
		return false
	}

	if isWindows() {
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
	} else {
		proc, _ := os.FindProcess(-pid)
		if proc != nil {
			_ = proc.Signal(os.Interrupt)
		} else {
			proc, _ = os.FindProcess(pid)
			_ = proc.Signal(os.Interrupt)
		}
	}

	a.RemovePID()
	a.RemoveStopFile()
	return true
}

func (a *App) RemoveStopFile() {
	os.Remove(a.stopFile())
}

func (a *App) StopFileExists() bool {
	_, err := os.Stat(a.stopFile())
	return err == nil
}

func (a *App) OpenLogFile() (*os.File, error) {
	if err := a.EnsureRootDir(); err != nil {
		return nil, err
	}
	return os.OpenFile(a.logFile(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}

func (a *App) LogFilePath() string {
	return a.logFile()
}

func (a *App) IsRunning() (bool, string, error) {
	if err := a.EnsureRootDir(); err != nil {
		return false, "", err
	}
	b, err := os.ReadFile(a.stateFile())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, "", nil
		}
		return false, "", err
	}
	state := strings.TrimSpace(string(b))
	return state == "running", state, nil
}

func (a *App) SaveState(state string) error {
	if err := a.EnsureRootDir(); err != nil {
		return err
	}
	return os.WriteFile(a.stateFile(), []byte(state+"\n"), 0o644)
}

func (a *App) ServerURL() (string, error) {
	if err := a.EnsureRootDir(); err != nil {
		return "", err
	}
	b, err := os.ReadFile(a.serverFile())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func (a *App) SaveServerURL(raw string) error {
	if err := a.EnsureRootDir(); err != nil {
		return err
	}
	if raw == "" {
		os.Remove(a.serverFile())
		return nil
	}
	return os.WriteFile(a.serverFile(), []byte(raw+"\n"), 0o644)
}

func isWindows() bool {
	return filepath.IsAbs(`C:\`)
}

func forceKill(pid int) {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = proc.Kill()
}
