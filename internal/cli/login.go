package cli

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"cs-cloud/internal/app"
	"cs-cloud/internal/device"
	"cs-cloud/internal/provider"
)

func login(a *app.App) error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cred, err := provider.LoginCoStrict(ctx)
	if err != nil {
		return err
	}

	printTitle("cs-cloud login")
	printSuccess("Login successful")
	printKV("machine_id", cred.MachineID)
	printKV("base_url", cred.BaseURL)

	info, err := device.Register(ctx, a.Config())
	if err != nil {
		if device.IsMissingAuthError(err) || device.IsExpiredAuthError(err) {
			printWarn("Authentication issue, please try login again")
			return err
		}
		printWarn("Device registration failed: %v", err)
		return nil
	}
	printSuccess("Device registered")
	printKV("device_id", info.DeviceID)
	return nil
}

func logout(a *app.App) error {
	if err := provider.DeleteCredentials(); err != nil {
		return fmt.Errorf("delete credentials: %w", err)
	}
	if err := device.ClearDevice(); err != nil {
		return fmt.Errorf("clear device: %w", err)
	}
	printSuccess("Logged out")
	return nil
}
