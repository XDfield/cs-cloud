package app

import (
	"fmt"
	"os"
	"path/filepath"

	"cs-cloud/internal/config"
	"cs-cloud/internal/device"
	"cs-cloud/internal/provider"
)

type App struct {
	rootDir string
	cfg     *config.Config
}

func New() (*App, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user home: %w", err)
	}
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	return &App{rootDir: filepath.Join(home, ".costrict", "cs-cloud"), cfg: cfg}, nil
}

func (a *App) RootDir() string  { return a.rootDir }
func (a *App) Config() *config.Config { return a.cfg }

func (a *App) EnsureRootDir() error {
	return os.MkdirAll(a.rootDir, 0o755)
}

func (a *App) CloudBaseURL() string {
	client := device.NewClient(a.cfg)
	return client.CloudBaseURL()
}

func (a *App) Credentials() (*provider.Credentials, error) {
	return provider.LoadCredentials()
}

func (a *App) Device() (*device.DeviceInfo, error) {
	return device.LoadDevice()
}
