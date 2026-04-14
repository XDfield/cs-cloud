package localserver

import (
	"context"
	"net"
	"net/http"
	"time"

	"cs-cloud/internal/agent"
	"cs-cloud/internal/logger"
	"cs-cloud/internal/runtime"
	"cs-cloud/internal/terminal"
)

type Server struct {
	http    *http.Server
	ln      net.Listener
	url     string
	version string

	manager    *runtime.AgentManager
	eventBus   *runtime.EventBus
	termMgr    *terminal.TerminalManager
	termH      *terminal.Handlers
	inputWsH   *terminal.InputWsHandler
}

func New(opts ...Option) *Server {
	initStartTime()

	s := &Server{
		eventBus: runtime.NewEventBus(),
	}
	for _, o := range opts {
		o(s)
	}
	s.manager = runtime.NewAgentManager(s.eventBus)

	s.termMgr = terminal.NewManager()
	s.termH = terminal.NewHandlers(s.termMgr)
	s.inputWsH = terminal.NewInputWsHandler(s.termMgr)

	mux := http.NewServeMux()
	api := http.NewServeMux()
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", api))

	api.HandleFunc("GET /runtime/health", s.handleHealth)
	api.HandleFunc("GET /runtime/files", s.handleFileList)
	api.HandleFunc("GET /runtime/files/content", s.handleFileContent)
	api.HandleFunc("GET /runtime/find/file", s.handleFindFiles)
	api.HandleFunc("GET /runtime/path", s.handlePath)
	api.HandleFunc("GET /runtime/vcs", s.handleVcs)
	api.HandleFunc("POST /runtime/dispose", s.handleInstanceDispose)

	api.HandleFunc("GET /agents", s.handleListAgents)
	api.HandleFunc("GET /agents/health", s.handleAgentHealth)
	api.HandleFunc("GET /agents/models", s.handleProxy)
	api.HandleFunc("GET /agents/session-modes", s.handleProxy)
	api.HandleFunc("GET /agents/commands", s.handleProxy)
	api.HandleFunc("GET /agents/mcp", s.handleProxy)
	api.HandleFunc("GET /agents/lsp", s.handleProxy)

	api.HandleFunc("POST /conversations", s.handleProxy)
	api.HandleFunc("GET /conversations", s.handleProxy)
	api.HandleFunc("GET /conversations/status", s.handleProxy)
	api.HandleFunc("GET /conversations/{id}", s.handleProxy)
	api.HandleFunc("PATCH /conversations/{id}", s.handleProxy)
	api.HandleFunc("DELETE /conversations/{id}", s.handleProxy)
	api.HandleFunc("POST /conversations/{id}/prompt", s.handleProxy)
	api.HandleFunc("POST /conversations/{id}/prompt/async", s.handleProxy)
	api.HandleFunc("POST /conversations/{id}/abort", s.handleProxy)
	api.HandleFunc("GET /conversations/{id}/messages", s.handleProxy)
	api.HandleFunc("GET /conversations/{id}/todo", s.handleProxy)
	api.HandleFunc("GET /conversations/{id}/diff", s.handleProxy)
	api.HandleFunc("POST /conversations/{id}/shell", s.handleProxy)
	api.HandleFunc("POST /conversations/{id}/command", s.handleProxy)

	api.HandleFunc("GET /events", s.handleProxy)

	api.HandleFunc("GET /permissions", s.handleProxy)
	api.HandleFunc("POST /permissions/{id}/reply", s.handleProxy)

	api.HandleFunc("GET /questions", s.handleProxy)
	api.HandleFunc("POST /questions/{id}/reply", s.handleProxy)
	api.HandleFunc("POST /questions/{id}/reject", s.handleProxy)

	api.HandleFunc("POST /terminal", s.termH.HandleCreate)
	api.HandleFunc("DELETE /terminal/{id}", s.termH.HandleKill)
	api.HandleFunc("POST /terminal/{id}/resize", s.termH.HandleResize)
	api.HandleFunc("POST /terminal/{id}/restart", s.termH.HandleRestart)
	api.HandleFunc("GET /terminal/{id}/stream", s.termH.HandleStream)
	api.HandleFunc("POST /terminal/{id}/input", s.termH.HandleInput)
	api.HandleFunc("GET /terminal/input-ws", s.inputWsH.ServeHTTP)

	s.http = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}
	return s
}

type Option func(*Server)

func WithVersion(v string) Option {
	return func(s *Server) { s.version = v }
}

func (s *Server) Manager() *runtime.AgentManager {
	return s.manager
}

func (s *Server) EventBus() *runtime.EventBus {
	return s.eventBus
}

func (s *Server) Start(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.ln = ln
	s.url = "http://" + ln.Addr().String()
	go func() {
		_ = s.http.Serve(ln)
	}()
	return nil
}

func (s *Server) URL() string {
	return s.url
}

func (s *Server) Port() int {
	if s.ln == nil {
		return 0
	}
	return s.ln.Addr().(*net.TCPAddr).Port
}

func (s *Server) Shutdown(ctx context.Context) error {
	s.termMgr.CloseAll()
	return s.http.Shutdown(ctx)
}

func (s *Server) TerminalManager() *terminal.TerminalManager {
	return s.termMgr
}

func (s *Server) InitDrivers(ctx context.Context) {
	d := agent.NewOpenCodeDriver()
	s.manager.RegisterDriver(d)
	logger.Info("registered opencode driver (cli=%s)", agent.OpenCodeCLIBinary)

	detected, _ := d.Detect(ctx)
	if len(detected) > 0 && detected[0].Available {
		var extra map[string]any
		if m, ok := detected[0].Extra.(map[string]any); ok {
			extra = m
		}
		cfg := agent.AgentConfig{
			ID:         "default",
			Backend:    "opencode",
			DriverName: "http",
			WorkingDir: "",
			Extra:      extra,
		}
		if err := s.manager.CreateAgent(ctx, "default", cfg); err != nil {
			logger.Error("failed to start opencode agent: %v", err)
		} else {
			logger.Info("opencode agent started (endpoint=%s)", s.manager.Endpoint())
		}
	}
}
