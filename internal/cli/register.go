package cli

import (
	"context"
	"fmt"

	"cs-cloud/internal/app"
	"cs-cloud/internal/device"
)

func register(a *app.App) error {
	ctx := context.Background()
	info, err := device.Register(ctx, a.Config())
	if err != nil {
		return err
	}
	fmt.Printf("device registered\ndevice_id: %s\nbase_url: %s\n", info.DeviceID, info.BaseURL)
	return nil
}
