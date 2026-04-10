package device

import (
	"context"

	"cs-cloud/internal/config"
)

func Register(ctx context.Context, cfg *config.Config) (*DeviceInfo, error) {
	client := NewClient(cfg)
	return client.Register(ctx)
}
