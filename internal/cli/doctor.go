package cli

import (
	"fmt"

	"cs-cloud/internal/app"
	"cs-cloud/internal/provider"
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
	hasCred := cred != nil
	hasDev := dev != nil
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
	}
	fmt.Printf("doctor: ok\nrunning: %t\nroot: %s\ncloud_base_url: %s\nauth_json_loaded: %t\ncredential_base_url: %s\nmachine_id: %s\ndevice_registered: %t\ndevice_id: %s\ndevice_base_url: %s\nlocal_server_url: %s\n", running, a.RootDir(), a.CloudBaseURL(), hasCred, credBaseURL, machineID, hasDev, deviceID, deviceBaseURL, serverURL)
	return nil
}
