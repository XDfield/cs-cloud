package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cs-cloud/internal/app"
	"cs-cloud/internal/localserver"
)

func serve(a *app.App) error {
	srv := localserver.New()
	if err := srv.Start("127.0.0.1:0"); err != nil {
		return err
	}
	if err := a.SaveServerURL(srv.URL()); err != nil {
		return err
	}
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
	<-ch
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return srv.Shutdown(ctx)
}
