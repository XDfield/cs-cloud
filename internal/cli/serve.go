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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := localserver.New(localserver.WithVersion(version.Get()))

	if err := srv.Manager().InitDefaultAgent(ctx); err != nil {
		printError("Failed to init agent: %v", err)
	} else {
		printSuccess("Agent started (endpoint=%s)", srv.Manager().Endpoint())
	}

	if err := srv.Start("127.0.0.1:0"); err != nil {
		return err
	}
	if err := a.SaveServerURL(srv.URL()); err != nil {
		return err
	}

	printTitle("cs-cloud serve")
	printSuccess("Server running")
	printKV("url", srv.URL())

	agents, _ := srv.Manager().DetectAgents(ctx)
	for _, ag := range agents {
		if ag.Available {
			printSuccess("Agent detected: %s (%s)", ag.Name, ag.Backend)
		}
	}

	printInfo("Press Ctrl+C to stop")

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer shutdownCancel()

	fmt.Println()
	printInfo("Shutting down...")
	return srv.Shutdown(shutdownCtx)
}
