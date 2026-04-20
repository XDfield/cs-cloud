package cs

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"

	"cs-cloud/internal/agent"
)

type Driver struct {
	cliPath string
}

func NewDriver(cliPath string) *Driver {
	if cliPath == "" {
		cliPath = CLIBinary
	}
	return &Driver{cliPath: cliPath}
}

func (d *Driver) Name() string { return "cs" }

func (d *Driver) Detect(ctx context.Context) ([]agent.DetectedAgent, error) {
	p, err := exec.LookPath(d.cliPath)
	if err != nil {
		return nil, nil
	}
	return []agent.DetectedAgent{
		{
			Backend:   "cs",
			Name:      "CS",
			Driver:    "http",
			Available: true,
			Extra: map[string]any{
				"cli_path": p,
			},
		},
	}, nil
}

func (d *Driver) CreateAgent(cfg agent.AgentConfig) (agent.Agent, error) {
	cliPath := d.cliPath
	if extra := cfg.Extra; extra != nil {
		if p, ok := extra["cli_path"].(string); ok && p != "" {
			cliPath = p
		}
	}
	a := NewAgent(agent.AgentConfig{
		ID:         cfg.ID,
		Backend:    "cs",
		DriverName: "http",
		WorkingDir: cfg.WorkingDir,
		CustomEnv:  cfg.CustomEnv,
		Extra: map[string]any{
			"cli_path": cliPath,
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
		{http.MethodPost, "/conversations", agent.RewriteTo("/session"), nil},
		{http.MethodGet, "/conversations", agent.RewriteTo("/session"), nil},
		{http.MethodGet, "/conversations/status", agent.RewriteTo("/session/status"), nil},
		{http.MethodGet, "/conversations/{id}", agent.RewriteSessionID("/session/"), nil},
		{http.MethodPatch, "/conversations/{id}", agent.RewriteSessionID("/session/"), nil},
		{http.MethodDelete, "/conversations/{id}", agent.RewriteSessionID("/session/"), nil},
		{http.MethodPost, "/conversations/{id}/prompt", agent.RewriteSessionIDWithSuffix("/session/", "/prompt_async"), agent.TransformPromptBody},
		{http.MethodPost, "/conversations/{id}/prompt/async", agent.RewriteSessionIDWithSuffix("/session/", "/prompt_async"), agent.TransformPromptBody},
		{http.MethodPost, "/conversations/{id}/abort", agent.RewriteSessionIDWithSuffix("/session/", "/abort"), nil},
		{http.MethodGet, "/conversations/{id}/messages", agent.RewriteSessionIDWithSuffix("/session/", "/message"), nil},
		{http.MethodGet, "/conversations/{id}/todo", agent.RewriteSessionIDWithSuffix("/session/", "/todo"), nil},
		{http.MethodGet, "/conversations/{id}/diff", agent.RewriteSessionIDWithSuffix("/session/", "/diff"), nil},
		{http.MethodPost, "/conversations/{id}/shell", agent.RewriteSessionIDWithSuffix("/session/", "/shell"), nil},
		{http.MethodPost, "/conversations/{id}/command", agent.RewriteSessionIDWithSuffix("/session/", "/command"), nil},
		{http.MethodGet, "/permissions", agent.RewriteTo("/permission"), nil},
		{http.MethodPost, "/permissions/{id}/reply", agent.RewritePermReply, agent.RenameJSONField("decision", "reply")},
		{http.MethodGet, "/questions", agent.RewriteTo("/question"), nil},
		{http.MethodPost, "/questions/{id}/reply", agent.RewriteQuestionAction("/reply"), nil},
		{http.MethodPost, "/questions/{id}/reject", agent.RewriteQuestionAction("/reject"), nil},
		{http.MethodGet, "/events", agent.RewriteTo("/event"), nil},
		{http.MethodGet, "/agents/models", agent.RewriteTo("/provider/capabilities"), nil},
		{http.MethodGet, "/agents/session-modes", agent.RewriteTo("/agent"), nil},
		{http.MethodGet, "/agents/commands", agent.RewriteTo("/command"), nil},
		{http.MethodGet, "/agents/mcp", agent.RewriteTo("/mcp"), nil},
		{http.MethodGet, "/agents/lsp", agent.RewriteTo("/lsp"), nil},
	}
}
