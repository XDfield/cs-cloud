package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"cs-cloud/internal/app"
	"cs-cloud/internal/device"
	"cs-cloud/internal/platform"
	"cs-cloud/internal/provider"
)

const readyTimeout = 30 * time.Second

func start(a *app.App) error {
	running, pid := a.DaemonStatus()
	if running {
		url, _ := a.ServerURL()
		fmt.Printf("cs-cloud already running (pid: %d)\nlocal_server_url: %s\n", pid, url)
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mode := parseMode()

	if mode == "cloud" {
		info, err := registerWithLogin(ctx, a)
		if err != nil {
			return err
		}
		fmt.Printf("device registered: %s\n", info.DeviceID)

		if err := device.ValidateDeviceToken(ctx, info); err != nil {
			if device.IsInvalidDeviceTokenError(err) {
				fmt.Println("device token is invalid, regenerating...")
				_ = device.ClearDevice()
				info, err = registerWithLogin(ctx, a)
				if err != nil {
					return err
				}
				fmt.Printf("device re-registered: %s\n", info.DeviceID)
				if err := device.ValidateDeviceToken(ctx, info); err != nil {
					return err
				}
			} else {
				return err
			}
		}
		fmt.Println("device token validated")
	}

	_ = a.SaveMode(mode)

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	logFd, err := a.OpenLogFile()
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}

	daemonArgs := []string{"_daemon"}
	if p := platform.AuthPath(); p != "" {
		daemonArgs = append(daemonArgs, "--auth-path", p)
	}

	cmd := exec.Command(exe, daemonArgs...)
	cmd.Stdout = logFd
	cmd.Stderr = logFd
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
	if err := cmd.Start(); err != nil {
		logFd.Close()
		return fmt.Errorf("start daemon: %w", err)
	}

	if err := a.WritePID(cmd.Process.Pid); err != nil {
		logFd.Close()
		return err
	}

	ready := false
	deadline := time.Now().Add(readyTimeout)
	for time.Now().Before(deadline) {
		if url, _ := a.ServerURL(); url != "" {
			ready = true
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	logFd.Close()

	if !ready {
		fmt.Println("cs-cloud daemon failed to start, check logs: " + filepath.Join(a.RootDir(), "app.log"))
		os.Exit(1)
	}

	url, _ := a.ServerURL()
	fmt.Printf("cs-cloud started (pid: %d, mode: %s)\nlocal_server_url: %s\nlogs: %s\n",
		cmd.Process.Pid, mode, url, filepath.Join(a.RootDir(), "app.log"))
	return nil
}

func registerWithLogin(ctx context.Context, a *app.App) (*device.DeviceInfo, error) {
	info, err := device.Register(ctx, a.Config())
	if err != nil {
		if device.IsMissingAuthError(err) || device.IsExpiredAuthError(err) {
			fmt.Println("cloud registration requires CoStrict login, starting login flow...")
			if _, loginErr := provider.LoginCoStrict(ctx); loginErr != nil {
				return nil, loginErr
			}
			fmt.Println("CoStrict login completed")
			info, err = device.Register(ctx, a.Config())
		}
		if device.IsInvalidDeviceTokenError(err) {
			_ = device.ClearDevice()
			if device.IsMissingAuthError(err) || device.IsExpiredAuthError(err) {
				_, _ = provider.LoginCoStrict(ctx)
			}
			info, err = device.Register(ctx, a.Config())
		}
		if err != nil {
			return nil, err
		}
	}
	return info, nil
}
