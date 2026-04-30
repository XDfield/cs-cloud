package cli

import (
	"fmt"
	"path/filepath"

	"cs-cloud/internal/app"
	"cs-cloud/internal/provider"
)

func status(a *app.App) error {
	running, pid := a.DaemonStatus()
	cred, err := provider.LoadCredentials()
	if err != nil {
		return err
	}
	dev, err := a.Device()
	if err != nil {
		return err
	}
	mode := a.LoadMode()
	serverURL, err := a.ServerURL()
	if err != nil {
		return err
	}

	printTitle("cs-cloud status")

	deviceIDVal := ""
	if dev != nil {
		deviceIDVal = dev.DeviceID
	}

	if running {
		printSuccess("Running")
		fmt.Print(renderKV([][2]string{
			{"pid", fmt.Sprintf("%d", pid)},
			{"mode", mode},
			{"root", a.RootDir()},
			{"cloud_url", a.CloudBaseURL()},
			{"auth", fmt.Sprintf("%t", cred != nil)},
			{"device", fmt.Sprintf("%t", dev != nil)},
			{"device_id", deviceIDVal},
			{"local_url", serverURL},
			{"logs", filepath.Join(a.RootDir(), "app.log")},
		}))
	} else {
		printInfo("Stopped")
		fmt.Print(renderKV([][2]string{
			{"root", a.RootDir()},
			{"cloud_url", a.CloudBaseURL()},
			{"auth", fmt.Sprintf("%t", cred != nil)},
			{"device", fmt.Sprintf("%t", dev != nil)},
			{"device_id", deviceIDVal},
		}))
	}
	return nil
}
