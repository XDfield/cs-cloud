package agent

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
)

type OpenCodeDriver struct {
	cliPath string
}

func NewOpenCodeDriver() *OpenCodeDriver {
	return &OpenCodeDriver{cliPath: OpenCodeCLIBinary}
}

func (d *OpenCodeDriver) Name() string { return "opencode" }

func (d *OpenCodeDriver) Detect(ctx context.Context) ([]DetectedAgent, error) {
	p, err := exec.LookPath(d.cliPath)
	if err != nil {
		return nil, nil
	}
	return []DetectedAgent{
		{
			Backend:   "opencode",
			Name:      "OpenCode",
			Driver:    "http",
			Available: true,
			Extra: map[string]any{
				"cli_path": p,
			},
		},
	}, nil
}

func (d *OpenCodeDriver) CreateAgent(cfg AgentConfig) (Agent, error) {
	cliPath := d.cliPath
	if extra := cfg.Extra; extra != nil {
		if p, ok := extra["cli_path"].(string); ok && p != "" {
			cliPath = p
		}
	}
	a := NewOpenCodeAgent(AgentConfig{
		ID:         cfg.ID,
		Backend:    "opencode",
		DriverName: "http",
		WorkingDir: cfg.WorkingDir,
		Extra: map[string]any{
			"cli_path": cliPath,
		},
	})
	return a, nil
}

func (d *OpenCodeDriver) HealthCheck(ctx context.Context, backend string) (*HealthResult, error) {
	return nil, fmt.Errorf("health check not supported in spawn mode")
}

func (d *OpenCodeDriver) ProxyRoutes() []ProxyRoute {
	return []ProxyRoute{
		{http.MethodPost, "/conversations", rewriteTo("/session")},
		{http.MethodGet, "/conversations", rewriteTo("/session")},
		{http.MethodGet, "/conversations/status", rewriteTo("/session/status")},
		{http.MethodGet, "/conversations/{id}", rewriteSessionID("/session/")},
		{http.MethodPatch, "/conversations/{id}", rewriteSessionID("/session/")},
		{http.MethodDelete, "/conversations/{id}", rewriteSessionID("/session/")},
		{http.MethodPost, "/conversations/{id}/prompt", rewriteSessionID("/session/")},
		{http.MethodPost, "/conversations/{id}/prompt/async", rewriteSessionIDWithSuffix("/session/", "/prompt_async")},
		{http.MethodPost, "/conversations/{id}/abort", rewriteSessionIDWithSuffix("/session/", "/abort")},
		{http.MethodGet, "/conversations/{id}/messages", rewriteSessionIDWithSuffix("/session/", "/message")},
		{http.MethodGet, "/conversations/{id}/todo", rewriteSessionIDWithSuffix("/session/", "/todo")},
		{http.MethodGet, "/conversations/{id}/diff", rewriteSessionIDWithSuffix("/session/", "/diff")},
		{http.MethodPost, "/conversations/{id}/shell", rewriteSessionIDWithSuffix("/session/", "/shell")},
		{http.MethodPost, "/conversations/{id}/command", rewriteSessionIDWithSuffix("/session/", "/command")},
		{http.MethodGet, "/permissions", rewriteTo("/permission")},
		{http.MethodPost, "/permissions/{id}/reply", rewritePermReply},
		{http.MethodGet, "/questions", rewriteTo("/question")},
		{http.MethodPost, "/questions/{id}/reply", rewriteQuestionAction("/reply")},
		{http.MethodPost, "/questions/{id}/reject", rewriteQuestionAction("/reject")},
		{http.MethodGet, "/events", rewriteTo("/event")},
		{http.MethodGet, "/agents/models", rewriteTo("/provider/capabilities")},
		{http.MethodGet, "/agents/session-modes", rewriteTo("/agent")},
		{http.MethodGet, "/agents/commands", rewriteTo("/command")},
		{http.MethodGet, "/agents/mcp", rewriteTo("/mcp")},
		{http.MethodGet, "/agents/lsp", rewriteTo("/lsp")},
	}
}

func rewriteTo(target string) func(map[string]string) string {
	return func(_ map[string]string) string { return target }
}

func rewriteSessionID(prefix string) func(map[string]string) string {
	return func(vals map[string]string) string {
		return prefix + vals["id"]
	}
}

func rewriteSessionIDWithSuffix(prefix, suffix string) func(map[string]string) string {
	return func(vals map[string]string) string {
		return prefix + vals["id"] + suffix
	}
}

func rewritePermReply(vals map[string]string) string {
	return "/permission/" + vals["id"] + "/reply"
}

func rewriteQuestionAction(suffix string) func(map[string]string) string {
	return func(vals map[string]string) string {
		return "/question/" + vals["id"] + suffix
	}
}
