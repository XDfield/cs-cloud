package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cs-cloud/internal/app"
	"cs-cloud/internal/localserver"
	"cs-cloud/internal/version"
)

func serve(a *app.App) error {
	srv := localserver.New(localserver.WithVersion(version.Get()))
	if err := srv.Start("127.0.0.1:0"); err != nil {
		return err
	}
	if err := a.SaveServerURL(srv.URL()); err != nil {
		return err
	}

	printTitle("cs-cloud serve")
	printSuccess("Server running")
	printKV("url", srv.URL())
	printInfo("Press Ctrl+C to stop")

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fmt.Println()
	printInfo("Shutting down...")
	return srv.Shutdown(ctx)
}
