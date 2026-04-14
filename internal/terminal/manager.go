package terminal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"cs-cloud/internal/config"
	"cs-cloud/internal/logger"
)

const defaultMaxSlots = 20

type TerminalManager struct {
	mu       sync.Mutex
	sessions map[string]*Session
	shell    string
	maxSlots int
	cfg      *config.Config
}

type Option func(*TerminalManager)

func WithMaxSlots(n int) Option {
	return func(m *TerminalManager) { m.maxSlots = n }
}

func WithConfig(cfg *config.Config) Option {
	return func(m *TerminalManager) { m.cfg = cfg }
}

func NewManager(opts ...Option) *TerminalManager {
	m := &TerminalManager{
		sessions: make(map[string]*Session),
		maxSlots: defaultMaxSlots,
	}
	for _, o := range opts {
		o(m)
	}
	m.shell = discoverShell(m.cfg)
	logger.Info("terminal: shell=%s, maxSlots=%d", m.shell, m.maxSlots)
	return m
}

func (m *TerminalManager) Create(cwd string, rows, cols uint16) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.sessions) >= m.maxSlots {
		return nil, fmt.Errorf("terminal: max sessions (%d) reached", m.maxSlots)
	}

	id := generateID()
	s, err := newSession(id, m.shell, cwd, rows, cols)
	if err != nil {
		return nil, fmt.Errorf("terminal: create session: %w", err)
	}

	m.sessions[id] = s
	logger.Info("terminal: session created id=%s pid=%d", id, s.Pid)
	return s, nil
}

func (m *TerminalManager) Get(id string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("terminal: session %s not found", id)
	}
	return s, nil
}

func (m *TerminalManager) Kill(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.sessions[id]
	if !ok {
		return fmt.Errorf("terminal: session %s not found", id)
	}

	s.Close()
	delete(m.sessions, id)
	logger.Info("terminal: session killed id=%s", id)
	return nil
}

func (m *TerminalManager) Resize(id string, rows, cols uint16) error {
	s, err := m.Get(id)
	if err != nil {
		return err
	}
	return s.Resize(rows, cols)
}

func (m *TerminalManager) Restart(id string, cwd string) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	old, ok := m.sessions[id]
	if !ok {
		return nil, fmt.Errorf("terminal: session %s not found", id)
	}

	if cwd == "" {
		cwd = old.Cwd
	}
	rows, cols := old.Rows, old.Cols

	old.Close()

	newS, err := newSession(id, m.shell, cwd, rows, cols)
	if err != nil {
		delete(m.sessions, id)
		return nil, fmt.Errorf("terminal: restart session: %w", err)
	}

	m.sessions[id] = newS
	logger.Info("terminal: session restarted id=%s pid=%d", id, newS.Pid)
	return newS, nil
}

func (m *TerminalManager) Write(id string, data []byte) error {
	s, err := m.Get(id)
	if err != nil {
		return err
	}
	return s.Write(data)
}

func (m *TerminalManager) Subscribe(id string) (<-chan []byte, func(), error) {
	s, err := m.Get(id)
	if err != nil {
		return nil, nil, err
	}
	ch, unsub := s.Subscribe()
	return ch, unsub, nil
}

func (m *TerminalManager) CleanupIdle(timeout time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, s := range m.sessions {
		if now.Sub(s.LastInput) > timeout {
			s.Close()
			delete(m.sessions, id)
			logger.Info("terminal: idle session cleaned up id=%s", id)
		}
	}
}

func (m *TerminalManager) CloseAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for id, s := range m.sessions {
		s.Close()
		delete(m.sessions, id)
	}
	logger.Info("terminal: all sessions closed")
}

func (m *TerminalManager) SessionCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sessions)
}

func newSession(id string, shell string, cwd string, rows, cols uint16) (*Session, error) {
	ptmx, pid, err := startPty(shell, cwd, rows, cols)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())
	s := &Session{
		ID:          id,
		Pid:         pid,
		Cols:        cols,
		Rows:        rows,
		Cwd:         cwd,
		CreatedAt:    time.Now(),
		LastInput:   time.Now(),
		cancel:      cancel,
		subscribers: make(map[string]chan []byte),
		ptmx:        ptmx,
	}

	go s.readOutput(ctx)
	return s, nil
}

func discoverShell(cfg *config.Config) string {
	if cfg != nil && cfg.DefaultShell != "" {
		return cfg.DefaultShell
	}

	if sh := os.Getenv("CS_CLOUD_SHELL"); sh != "" {
		return sh
	}

	if runtime.GOOS == "windows" {
		return discoverWindowsShell()
	}
	return discoverUnixShell()
}

func discoverUnixShell() string {
	candidates := []string{os.Getenv("SHELL"), "/bin/zsh", "/bin/bash", "/bin/sh"}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "/bin/sh"
}

func discoverWindowsShell() string {
	candidates := []string{
		os.Getenv("ComSpec"),
	}
	if systemRoot := os.Getenv("SystemRoot"); systemRoot != "" {
		ps := systemRoot + `\System32\WindowsPowerShell\v1.0\powershell.exe`
		candidates = append(candidates, ps)
	}
	candidates = append(candidates, "pwsh", "powershell")
	if gitBash := findGitBash(); gitBash != "" {
		candidates = append(candidates, gitBash)
	}
	candidates = append(candidates, "cmd")
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return "cmd.exe"
}

func findGitBash() string {
	git, err := exec.LookPath("git")
	if err != nil {
		return ""
	}
	dir := filepath.Dir(filepath.Dir(git))
	bash := filepath.Join(dir, "bin", "bash.exe")
	if _, err := os.Stat(bash); err == nil {
		return bash
	}
	return ""
}
