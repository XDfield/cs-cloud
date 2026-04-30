package cli

import (
	"context"
	"fmt"
	"os"
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
		printWarn("cs-cloud is already running")
		printKV("pid", fmt.Sprintf("%d", pid))
		printKV("url", url)
		return nil
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	mode := parseMode()
	port, err := parsePort()
	if err != nil {
		return err
	}

	if mode == "cloud" {
		info, err := registerWithLogin(ctx, a)
		if err != nil {
			return err
		}
		printSuccess("Device registered")
		printKV("device_id", info.DeviceID)

		if err := device.ValidateDeviceToken(ctx, info); err != nil {
			if device.IsInvalidDeviceTokenError(err) {
				fmt.Println("device token is invalid, regenerating...")
				_ = device.ClearDevice()
				info, err = registerWithLogin(ctx, a)
				if err != nil {
					return err
				}
				printWarn("Device re-registered")
				printKV("device_id", info.DeviceID)
				if err := device.ValidateDeviceToken(ctx, info); err != nil {
					return err
				}
			} else {
				return err
			}
		}
		printSuccess("Device token validated")
	}

	_ = a.SaveMode(mode)

	printInfo("Starting daemon...")

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	logFd, err := a.OpenLogFile()
	if err != nil {
		return fmt.Errorf("open log file: %w", err)
	}
	defer logFd.Close()

	nullFd, err := openNullDevice()
	if err != nil {
		return fmt.Errorf("open null device: %w", err)
	}
	defer nullFd.Close()

	daemonArgs := []string{"_daemon"}
	if p := platform.AuthPath(); p != "" {
		daemonArgs = append(daemonArgs, "--auth-path", p)
	}
	if d := platform.DataDir(); d != "" {
		daemonArgs = append(daemonArgs, "--data-dir", d)
	}
	if port > 0 {
		daemonArgs = append(daemonArgs, "--port", fmt.Sprintf("%d", port))
	}
	if platform.NoAutoUpgrade() {
		daemonArgs = append(daemonArgs, "--no-auto-upgrade")
	}

	cmd := newDaemonCmd(exe, daemonArgs)
	cmd.Stdin = nullFd
	cmd.Stdout = logFd
	cmd.Stderr = logFd
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start daemon: %w", err)
	}

	if err := a.WritePID(cmd.Process.Pid); err != nil {
		return err
	}

	daemonExited := make(chan error, 1)
	go func() { daemonExited <- cmd.Wait() }()

	ready := false
	deadline := time.Now().Add(readyTimeout)
	lastDot := time.Now()
	dotCount := 0
	for time.Now().Before(deadline) {
		if url, _ := a.ServerURL(); url != "" {
			ready = true
			break
		}
		select {
		case <-daemonExited:
			goto waitDone
		default:
		}
		if time.Since(lastDot) >= 3*time.Second {
			fmt.Print(".")
			dotCount++
			lastDot = time.Now()
		}
		time.Sleep(200 * time.Millisecond)
	}
waitDone:
	if dotCount > 0 {
		fmt.Println()
	}

	if !ready {
		printError("Daemon failed to start")
		printInfo("Check logs: %s", filepath.Join(a.RootDir(), "app.log"))
		os.Exit(1)
	}

	url, _ := a.ServerURL()
	printSuccess("cs-cloud started")
	printKV("pid", fmt.Sprintf("%d", cmd.Process.Pid))
	printKV("mode", mode)
	printKV("url", url)
	printKV("docs", url+"/api/v1/docs")
	printKV("logs", filepath.Join(a.RootDir(), "app.log"))
	return nil
}

func registerWithLogin(ctx context.Context, a *app.App) (*device.DeviceInfo, error) {
	info, err := device.Register(ctx, a.Config())
	if err != nil {
		if device.IsMissingAuthError(err) || device.IsExpiredAuthError(err) || device.IsAuthError(err) {
			printInfo("Cloud registration requires CoStrict login, starting login flow...")
			if _, loginErr := provider.LoginCoStrict(ctx); loginErr != nil {
				return nil, loginErr
			}
			printSuccess("CoStrict login completed")
			info, err = device.Register(ctx, a.Config())
		}
		if device.IsInvalidDeviceTokenError(err) {
			_ = device.ClearDevice()
			if device.IsMissingAuthError(err) || device.IsExpiredAuthError(err) || device.IsAuthError(err) {
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
