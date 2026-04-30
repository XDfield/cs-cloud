package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"cs-cloud/internal/logger"
)

func SelfRestart(a *App) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve exe: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolve exe symlink: %w", err)
	}

	args := a.LoadArgs()
	if len(args) == 0 {
		args = []string{"_daemon"}
	}

	if err := a.SaveState("restarting"); err != nil {
		return fmt.Errorf("save state: %w", err)
	}

	logger.Info("[selfrestart] launching new process: %s %v", exe, args)

	cmd := newRestartCmd(exe, args)
	logFd, logErr := a.OpenLogFile()
	if logErr == nil {
		cmd.Stdout = logFd
		cmd.Stderr = logFd
	}

	if err := cmd.Start(); err != nil {
		a.SaveState("running")
		return fmt.Errorf("start new process: %w", err)
	}

	if logFd != nil {
		go func() {
			cmd.Wait()
			logFd.Close()
		}()
	}

	logger.Info("[selfrestart] new process started (pid=%d), exiting current", cmd.Process.Pid)
	os.Exit(0)
	return nil
}

func newRestartCmd(exe string, args []string) *exec.Cmd {
	return newDaemonCmd(exe, args)
}
