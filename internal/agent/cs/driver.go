package cs

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"

	"cs-cloud/internal/agent"
)

type Driver struct {
	cmd agent.Command
}

func NewDriver(cmd agent.Command) *Driver {
	return &Driver{cmd: cmd}
}

func (d *Driver) Name() string { return "cs" }

func (d *Driver) Detect(ctx context.Context) ([]agent.DetectedAgent, error) {
	if d.cmd.IsZero() {
		return nil, nil
	}
	p, err := exec.LookPath(d.cmd.Binary())
	if err != nil {
		return nil, nil
	}
	_ = p
	return []agent.DetectedAgent{
		{
			Backend:   "cs",
			Name:      "CS",
			Driver:    "http",
			Available: true,
			Extra: map[string]any{
				"command": d.cmd,
			},
		},
	}, nil
}

func (d *Driver) CreateAgent(cfg agent.AgentConfig) (agent.Agent, error) {
	cmd := d.cmd
	if extra := cfg.Extra; extra != nil {
		if c, ok := extra["command"].(agent.Command); ok && !c.IsZero() {
			cmd = c
		}
	}
	a := NewAgent(agent.AgentConfig{
		ID:         cfg.ID,
		Backend:    "cs",
		DriverName: "http",
		WorkingDir: cfg.WorkingDir,
		CustomEnv:  cfg.CustomEnv,
		Extra: map[string]any{
			"command": cmd,
		},
	})
	return a, nil
}

func (d *Driver) HealthCheck(ctx context.Context, backend string) (*agent.HealthResult, error) {
	return nil, fmt.Errorf("health check not supported in spawn mode")
}

func (d *Driver) HeaderMap() map[string]string {
	return map[string]string{
		"X-Workspace-Directory": "x-opencode-directory",
	}
}

func (d *Driver) ProxyRoutes() []agent.ProxyRoute {
	return []agent.ProxyRoute{
		// todo: [统一接口到 cs-cloud start]
		{Method: http.MethodPost, Prefix: "/session", Rewrite: agent.RewriteTo("/session")},
		{Method: http.MethodGet, Prefix: "/session", Rewrite: agent.RewriteTo("/session")},
		{Method: http.MethodGet, Prefix: "/session/status", Rewrite: agent.RewriteTo("/session/status")},
		{Method: http.MethodGet, Prefix: "/agent", Rewrite: agent.RewriteTo("/agent")},
		{Method: http.MethodGet, Prefix: "/command", Rewrite: agent.RewriteTo("/command")},
		{Method: http.MethodGet, Prefix: "/provider", Rewrite: agent.RewriteTo("/provider")},
		{Method: http.MethodGet, Prefix: "/config/providers", Rewrite: agent.RewriteTo("/config/providers")},
		{Method: http.MethodGet, Prefix: "/find/file", Rewrite: agent.RewriteTo("/find/file")},
		{Method: http.MethodGet, Prefix: "/file", Rewrite: agent.RewriteTo("/file")},
		{Method: http.MethodGet, Prefix: "/file/content", Rewrite: agent.RewriteTo("/file/content")},
		{Method: http.MethodGet, Prefix: "/file/status", Rewrite: agent.RewriteTo("/file/status")},
		{Method: http.MethodGet, Prefix: "/experimental/session", Rewrite: agent.RewriteTo("/session")},
		{Method: http.MethodGet, Prefix: "/session/{id}", Rewrite: agent.RewriteSessionID("/session/")},
		{Method: http.MethodPatch, Prefix: "/session/{id}", Rewrite: agent.RewriteSessionID("/session/")},
		{Method: http.MethodDelete, Prefix: "/session/{id}", Rewrite: agent.RewriteSessionID("/session/")},
		{Method: http.MethodPost, Prefix: "/session/{id}/prompt", Rewrite: agent.RewriteSessionIDWithSuffix("/session/", "/prompt")},
		{Method: http.MethodPost, Prefix: "/session/{id}/prompt_async", Rewrite: agent.RewriteSessionIDWithSuffix("/session/", "/prompt_async")},
		{Method: http.MethodPost, Prefix: "/session/{id}/abort", Rewrite: agent.RewriteSessionIDWithSuffix("/session/", "/abort")},
		{Method: http.MethodPost, Prefix: "/session/{id}/summarize", Rewrite: agent.RewriteSessionIDWithSuffix("/session/", "/summarize")},
		{Method: http.MethodPost, Prefix: "/session/{id}/revert", Rewrite: agent.RewriteSessionIDWithSuffix("/session/", "/revert")},
		{Method: http.MethodPost, Prefix: "/session/{id}/unrevert", Rewrite: agent.RewriteSessionIDWithSuffix("/session/", "/unrevert")},
		{Method: http.MethodPost, Prefix: "/session/{id}/fork", Rewrite: agent.RewriteSessionIDWithSuffix("/session/", "/fork")},
		{Method: http.MethodGet, Prefix: "/session/{id}/message", Rewrite: agent.RewriteSessionIDWithSuffix("/session/", "/message")},
		{Method: http.MethodGet, Prefix: "/session/{id}/todo", Rewrite: agent.RewriteSessionIDWithSuffix("/session/", "/todo")},
		{Method: http.MethodGet, Prefix: "/session/{id}/diff", Rewrite: agent.RewriteSessionIDWithSuffix("/session/", "/diff")},
		{Method: http.MethodPost, Prefix: "/session/{id}/shell", Rewrite: agent.RewriteSessionIDWithSuffix("/session/", "/shell")},
		{Method: http.MethodPost, Prefix: "/session/{id}/command", Rewrite: agent.RewriteSessionIDWithSuffix("/session/", "/command")},
		{Method: http.MethodGet, Prefix: "/permission", Rewrite: agent.RewriteTo("/permission")},
		{Method: http.MethodPost, Prefix: "/permission/{id}/reply", Rewrite: agent.RewritePermReply},
		{Method: http.MethodGet, Prefix: "/question", Rewrite: agent.RewriteTo("/question")},
		{Method: http.MethodPost, Prefix: "/question/{id}/reply", Rewrite: agent.RewriteQuestionAction("/reply")},
		{Method: http.MethodPost, Prefix: "/question/{id}/reject", Rewrite: agent.RewriteQuestionAction("/reject")},
		{Method: http.MethodGet, Prefix: "/event", Rewrite: agent.RewriteTo("/event")},
		// [统一接口到 cs-cloud end]

		{Method: http.MethodPost, Prefix: "/conversations", Rewrite: agent.RewriteTo("/session")},
		{Method: http.MethodGet, Prefix: "/conversations", Rewrite: agent.RewriteTo("/session")},
		{Method: http.MethodGet, Prefix: "/conversations/status", Rewrite: agent.RewriteTo("/session/status")},
		{Method: http.MethodGet, Prefix: "/conversations/{id}", Rewrite: agent.RewriteSessionID("/session/")},
		{Method: http.MethodPatch, Prefix: "/conversations/{id}", Rewrite: agent.RewriteSessionID("/session/")},
		{Method: http.MethodDelete, Prefix: "/conversations/{id}", Rewrite: agent.RewriteSessionID("/session/")},
		{Method: http.MethodPost, Prefix: "/conversations/{id}/prompt", Rewrite: agent.RewriteSessionIDWithSuffix("/session/", "/prompt_async"), Transform: agent.TransformPromptBody},
		{Method: http.MethodPost, Prefix: "/conversations/{id}/prompt/async", Rewrite: agent.RewriteSessionIDWithSuffix("/session/", "/prompt_async"), Transform: agent.TransformPromptBody},
		{Method: http.MethodPost, Prefix: "/conversations/{id}/abort", Rewrite: agent.RewriteSessionIDWithSuffix("/session/", "/abort")},
		{Method: http.MethodGet, Prefix: "/conversations/{id}/messages", Rewrite: agent.RewriteSessionIDWithSuffix("/session/", "/message")},
		{Method: http.MethodGet, Prefix: "/conversations/{id}/todo", Rewrite: agent.RewriteSessionIDWithSuffix("/session/", "/todo")},
		{Method: http.MethodGet, Prefix: "/conversations/{id}/diff", Rewrite: agent.RewriteSessionIDWithSuffix("/session/", "/diff")},
		{Method: http.MethodPost, Prefix: "/conversations/{id}/shell", Rewrite: agent.RewriteSessionIDWithSuffix("/session/", "/shell")},
		{Method: http.MethodPost, Prefix: "/conversations/{id}/command", Rewrite: agent.RewriteSessionIDWithSuffix("/session/", "/command")},
		{Method: http.MethodGet, Prefix: "/permissions", Rewrite: agent.RewriteTo("/permission")},
		{Method: http.MethodPost, Prefix: "/permissions/{id}/reply", Rewrite: agent.RewritePermReply, Transform: agent.RenameJSONField("decision", "reply")},
		{Method: http.MethodGet, Prefix: "/questions", Rewrite: agent.RewriteTo("/question")},
		{Method: http.MethodPost, Prefix: "/questions/{id}/reply", Rewrite: agent.RewriteQuestionAction("/reply")},
		{Method: http.MethodPost, Prefix: "/questions/{id}/reject", Rewrite: agent.RewriteQuestionAction("/reject")},
		{Method: http.MethodGet, Prefix: "/events", Rewrite: agent.RewriteTo("/event")},
		{Method: http.MethodGet, Prefix: "/agents/models", Rewrite: agent.RewriteTo("/provider/capabilities")},
		{Method: http.MethodGet, Prefix: "/agents/session-modes", Rewrite: agent.RewriteTo("/agent")},
		{Method: http.MethodGet, Prefix: "/agents/commands", Rewrite: agent.RewriteTo("/command")},
		{Method: http.MethodGet, Prefix: "/agents/mcp", Rewrite: agent.RewriteTo("/mcp")},
		{Method: http.MethodGet, Prefix: "/agents/lsp", Rewrite: agent.RewriteTo("/lsp")},
	}
}
