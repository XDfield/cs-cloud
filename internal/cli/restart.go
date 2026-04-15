package cli

import "cs-cloud/internal/app"

func restart(a *app.App) error {
	printInfo("Stopping previous instance...")
	stopped := a.StopDaemon()
	if stopped {
		printInfo("Stopped previous instance")
	}
	return start(a)
}
