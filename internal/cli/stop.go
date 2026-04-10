package cli

import (
	"fmt"

	"cs-cloud/internal/app"
)

func stop(a *app.App) error {
	stopped := a.StopDaemon()
	if stopped {
		fmt.Println("cs-cloud stopped")
	} else {
		fmt.Println("cs-cloud is not running")
	}
	return nil
}
