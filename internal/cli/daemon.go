package cli

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"cs-cloud/internal/app"
	"cs-cloud/internal/device"
	"cs-cloud/internal/localserver"
	"cs-cloud/internal/logger"
	"cs-cloud/internal/tunnel"
	"cs-cloud/internal/updater"
	"cs-cloud/internal/version"
)

func runDaemon(a *app.App) error {
	mode := a.LoadMode()

	logger.Init(logger.Config{
		Dir:        a.RootDir(),
		MaxSizeMB:  100,
		MaxAgeDays: 7,
		MaxBackups: 10,
		Console:    false,
	})
	defer logger.Sync()

	srv := localserver.New(localserver.WithVersion(version.Get()))

	ctx := context.Background()
	if err := srv.Manager().InitDefaultAgent(ctx); err != nil {
		logger.Error("failed to init agent: %v", err)
	} else {
		logger.Info("agent started (endpoint=%s)", srv.Manager().Endpoint())
	}

	if err := srv.Start("127.0.0.1:0"); err != nil {
		logger.Error("failed to start server: %v", err)
		return err
	}
	if err := a.SaveServerURL(srv.URL()); err != nil {
		logger.Error("failed to save server url: %v", err)
		return err
	}
	if err := a.SaveState("running"); err != nil {
		logger.Error("failed to save state: %v", err)
		return err
	}

	logger.Info("daemon started (version: %s, mode: %s, port: %d)", version.FullString(), mode, srv.Port())

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			srv.TerminalManager().CleanupIdle(30 * time.Minute)
		}
	}()

	if mode == "cloud" {
		info, err := device.LoadDevice()
		if err != nil || info == nil {
			logger.Error("device not registered")
			return nil
		}

		ctx := context.Background()
		go func() {
			if err := tunnel.Connect(ctx, srv.Port()); err != nil {
				logger.Error("tunnel error: %v", err)
			}
		}()

		mgr := updater.NewManager(a.CloudBaseURL(), a.RootDir())
		go mgr.Run(ctx)
	}

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	if runtime.GOOS == "windows" {
		a.RemoveStopFile()
		go func() {
			for {
				time.Sleep(500 * time.Millisecond)
				if a.StopFileExists() {
					shutdown <- syscall.SIGTERM
					return
				}
			}
		}()
	}

	<-shutdown
	logger.Info("daemon shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)

	a.SaveState("stopped")
	a.SaveServerURL("")
	a.RemovePID()

	logger.Info("daemon stopped")
	return nil
}
