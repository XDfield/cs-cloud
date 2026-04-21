package cli

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"

	"cs-cloud/internal/app"
	"cs-cloud/internal/device"
	"cs-cloud/internal/localserver"
	"cs-cloud/internal/logger"
	"cs-cloud/internal/tunnel"
	"cs-cloud/internal/version"
)


func prewarmRequest(ctx context.Context, cli *http.Client, base string, path string, dir string) {
	begin := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		logger.Warn("prewarm request build failed (%s): %v", path, err)
		return
	}
	req.Header.Set("x-opencode-directory", dir)

	resp, err := cli.Do(req)
	if err != nil {
		logger.Warn("prewarm request failed (%s) after %s: %v", path, time.Since(begin), err)
		return
	}
	resp.Body.Close()

	cost := time.Since(begin)
	if resp.StatusCode >= http.StatusBadRequest {
		logger.Warn("prewarm request returned %d (%s) in %s", resp.StatusCode, path, cost)
		return
	}
	logger.Info("prewarm request ok (%s) in %s", path, cost)
}

func prewarmServer(ctx context.Context, base string, dir string) {
	fast := []string{
		"/agent",
		"/command",
		"/provider/capabilities",
		"/vcs",
	}
	start := time.Now()
	logger.Info("server prewarm started (workspace=%s)", dir)
	prewarmRequest(ctx, &http.Client{Timeout: 30 * time.Second}, base, "/session", dir)

	var wg sync.WaitGroup
	for _, path := range fast {
		path := path
		wg.Add(1)
		go func() {
			defer wg.Done()
			prewarmRequest(ctx, &http.Client{Timeout: 15 * time.Second}, base, path, dir)
		}()
	}
	wg.Wait()
	logger.Info("server prewarm finished in %s", time.Since(start))
}

func prewarmRecent(ctx context.Context, base string, dirs []string) {
	const limit = 2
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		dir := dir
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			prewarmServer(ctx, base, dir)
		}()
	}
	wg.Wait()
}

func collectRecent(dirs []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		abs, err := filepath.Abs(filepath.Clean(dir))
		if err != nil {
			continue
		}
		if _, ok := seen[abs]; ok {
			continue
		}
		if info, err := os.Stat(abs); err != nil || !info.IsDir() {
			continue
		}
		seen[abs] = struct{}{}
		out = append(out, abs)
	}
	return out
}

func runDaemon(a *app.App) error {
	configureDaemonSignals()

	logger.Init(logger.Config{
		Dir:        a.RootDir(),
		MaxSizeMB:  100,
		MaxAgeDays: 7,
		MaxBackups: 10,
		Console:    false,
	})
	defer logger.Sync()

	logger.Info("[debug] daemon process started (pid=%d)", os.Getpid())

	mode := a.LoadMode()
	a.SaveArgs(os.Args[1:])

	logger.Info("[debug] initializing local server...")
	srv := localserver.New(localserver.WithVersion(version.Get()), localserver.WithConfig(a.Config()), localserver.WithRootDir(a.RootDir()))

	ctx := context.Background()
	agentType := a.Config().DefaultAgent
	agentCommand := a.Config().AgentCommand
	if agentType == "" {
		agentType = "cs"
	}
	logger.Info("[debug] detecting agent (type=%s, command=%q)...", agentType, agentCommand)
	if err := srv.Manager().InitDefaultAgent(ctx, agentType, agentCommand, a.Config().AgentWorkspace, a.Config().AgentEnv); err != nil {
		logger.Error("failed to init agent: %v", err)
		logger.Error("please check your agent_command configuration works correctly in your terminal")
		return err
	}
	logger.Info("agent started (endpoint=%s)", srv.Manager().Endpoint())

	logger.Info("[debug] agent init done, starting HTTP server...")

	if pid := srv.Manager().AgentPID(); pid > 0 {
		if err := a.WriteAgentPID(pid); err != nil {
			logger.Warn("failed to save agent pid: %v", err)
		}
	}

	if err := srv.Start("127.0.0.1:0"); err != nil {
		logger.Error("failed to start server: %v", err)
		return err
	}
	logger.Info("[debug] HTTP server started, saving state...")
	if err := a.SaveServerURL(srv.URL()); err != nil {
		logger.Error("failed to save server url: %v", err)
		return err
	}
	if err := a.SaveState("running"); err != nil {
		logger.Error("failed to save state: %v", err)
		return err
	}

	logger.Info("daemon started (version: %s, mode: %s, port: %d)", version.FullString(), mode, srv.Port())
	logger.Info("swagger docs: %s/api/v1/docs", srv.URL())
	recent, err := a.LoadRecentWorkspaces()
	if err != nil {
		logger.Warn("failed to load recent workspaces: %v", err)
	}
	dir, cwdErr := os.Getwd()
	if cwdErr != nil {
		logger.Warn("failed to resolve prewarm workspace: %v", cwdErr)
	} else {
		recent = append([]string{dir}, recent...)
	}
	recent = collectRecent(recent)
	if len(recent) > 0 {
		go prewarmRecent(context.Background(), srv.Manager().Endpoint(), recent)
	}

	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			srv.TerminalManager().CleanupIdle(30 * time.Minute)
		}
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

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
	}

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
	a.RemoveAgentPID()

	logger.Info("daemon stopped")
	return nil
}
