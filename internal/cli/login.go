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

	fmt.Printf("login successful\nmachine_id: %s\nbase_url: %s\n", cred.MachineID, cred.BaseURL)

	info, err := device.Register(ctx, a.Config())
	if err != nil {
		if device.IsMissingAuthError(err) || device.IsExpiredAuthError(err) {
			fmt.Println("authentication issue, please try login again")
			return err
		}
		fmt.Printf("warning: device registration failed: %v\n", err)
		return nil
	}
	fmt.Printf("device registered\ndevice_id: %s\n", info.DeviceID)
	return nil
}

func logout(a *app.App) error {
	if err := provider.DeleteCredentials(); err != nil {
		return fmt.Errorf("delete credentials: %w", err)
	}
	if err := device.ClearDevice(); err != nil {
		return fmt.Errorf("clear device: %w", err)
	}
	fmt.Println("logged out")
	return nil
}
