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
	hasCred := cred != nil
	hasDev := dev != nil
	base := a.CloudBaseURL()
	serverURL, err := a.ServerURL()
	if err != nil {
		return err
	}
	mode := a.LoadMode()
	if running {
		fmt.Printf("status: running\npid: %d\nmode: %s\nroot: %s\ncloud_base_url: %s\nauth_json_loaded: %t\ndevice_registered: %t\nlocal_server_url: %s\nlogs: %s\n", pid, mode, a.RootDir(), base, hasCred, hasDev, serverURL, filepath.Join(a.RootDir(), "app.log"))
		return nil
	}
	fmt.Printf("status: stopped\nroot: %s\ncloud_base_url: %s\nauth_json_loaded: %t\ndevice_registered: %t\n", a.RootDir(), base, hasCred, hasDev)
	return nil
}
