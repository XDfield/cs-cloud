package localserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

type Server struct {
	http   *http.Server
	ln     net.Listener
	url    string
	version string
}

func New(opts ...Option) *Server {
	initStartTime()

	s := &Server{}
	for _, o := range opts {
		o(s)
	}

	mux := http.NewServeMux()
	api := http.NewServeMux()
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", api))

	api.HandleFunc("GET /runtime/health", s.handleHealth)

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

func (s *Server) Start(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.ln = ln
	s.url = fmt.Sprintf("http://%s", ln.Addr().String())
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
	return s.http.Shutdown(ctx)
}
