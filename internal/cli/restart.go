package cli

import (
	"fmt"

	"cs-cloud/internal/app"
)

func restart(a *app.App) error {
	stopped := a.StopDaemon()
	if stopped {
		fmt.Println("cs-cloud stopped")
	}
	return start(a)
}
