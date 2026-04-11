package cli

import "cs-cloud/internal/app"

func stop(a *app.App) error {
	stopped := a.StopDaemon()
	if stopped {
		printSuccess("cs-cloud stopped")
	} else {
		printWarn("cs-cloud is not running")
	}
	return nil
}
