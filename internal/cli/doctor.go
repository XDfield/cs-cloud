package cli

import (
	"fmt"

	"cs-cloud/internal/app"
	"cs-cloud/internal/provider"
	"cs-cloud/internal/updater"
	"cs-cloud/internal/version"
)

func doctor(a *app.App) error {
	running, _, err := a.IsRunning()
	if err != nil {
		return err
	}
	cred, err := provider.LoadCredentials()
	if err != nil {
		return err
	}
	dev, err := a.Device()
	if err != nil {
		return err
	}
	serverURL, err := a.ServerURL()
	if err != nil {
		return err
	}

	machineID := ""
	credBaseURL := ""
	deviceID := ""
	deviceBaseURL := ""
	if cred != nil {
		machineID = cred.MachineID
		credBaseURL = cred.BaseURL
	}
	if dev != nil {
		deviceID = dev.DeviceID
		deviceBaseURL = dev.BaseURL
	} else {
		deviceID = provider.GenerateMachineID()
	}

	mgr := updater.NewManager(a.CloudBaseURL(), a.RootDir())
	upgradeStatus := "none"
	if state, _ := mgr.History(); state != nil {
		upgradeStatus = state.Status
	}

	printTitle("cs-cloud doctor")
	printSuccess("OK")
	fmt.Print(renderKV([][2]string{
		{"version", version.Get()},
		{"commit", version.Commit},
		{"running", fmt.Sprintf("%t", running)},
		{"root", a.RootDir()},
		{"cloud_url", a.CloudBaseURL()},
		{"auth", fmt.Sprintf("%t", cred != nil)},
		{"cred_base_url", credBaseURL},
		{"machine_id", machineID},
		{"device", fmt.Sprintf("%t", dev != nil)},
		{"device_id", deviceID},
		{"device_base_url", deviceBaseURL},
		{"local_url", serverURL},
		{"upgrade", upgradeStatus},
	}))
	return nil
}
