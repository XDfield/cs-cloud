package agent

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/exec"
)

type OpenCodeDriver struct {
	cliPath string
}

func NewOpenCodeDriver(cliPath string) *OpenCodeDriver {
	if cliPath == "" {
		cliPath = OpenCodeCLIBinary
	}
	return &OpenCodeDriver{cliPath: cliPath}
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
		CustomEnv:  cfg.CustomEnv,
		Extra: map[string]any{
			"cli_path": cliPath,
		},
	})
	return a, nil
}

func (d *OpenCodeDriver) HealthCheck(ctx context.Context, backend string) (*HealthResult, error) {
	return nil, fmt.Errorf("health check not supported in spawn mode")
}

func (d *OpenCodeDriver) HeaderMap() map[string]string {
	return map[string]string{
		"X-Workspace-Directory": "x-opencode-directory",
	}
}

func (d *OpenCodeDriver) ProxyRoutes() []ProxyRoute {
	return []ProxyRoute{
		{http.MethodPost, "/conversations", rewriteTo("/session"), nil},
		{http.MethodGet, "/conversations", rewriteTo("/session"), nil},
		{http.MethodGet, "/conversations/status", rewriteTo("/session/status"), nil},
		{http.MethodGet, "/conversations/{id}", rewriteSessionID("/session/"), nil},
		{http.MethodPatch, "/conversations/{id}", rewriteSessionID("/session/"), nil},
		{http.MethodDelete, "/conversations/{id}", rewriteSessionID("/session/"), nil},
		{http.MethodPost, "/conversations/{id}/prompt", rewriteSessionID("/session/"), nil},
		{http.MethodPost, "/conversations/{id}/prompt/async", rewriteSessionIDWithSuffix("/session/", "/prompt_async"), nil},
		{http.MethodPost, "/conversations/{id}/abort", rewriteSessionIDWithSuffix("/session/", "/abort"), nil},
		{http.MethodGet, "/conversations/{id}/messages", rewriteSessionIDWithSuffix("/session/", "/message"), nil},
		{http.MethodGet, "/conversations/{id}/todo", rewriteSessionIDWithSuffix("/session/", "/todo"), nil},
		{http.MethodGet, "/conversations/{id}/diff", rewriteSessionIDWithSuffix("/session/", "/diff"), nil},
		{http.MethodPost, "/conversations/{id}/shell", rewriteSessionIDWithSuffix("/session/", "/shell"), nil},
		{http.MethodPost, "/conversations/{id}/command", rewriteSessionIDWithSuffix("/session/", "/command"), nil},
		{http.MethodGet, "/permissions", rewriteTo("/permission"), nil},
		{http.MethodPost, "/permissions/{id}/reply", rewritePermReply, renameJSONField("decision", "reply")},
		{http.MethodGet, "/questions", rewriteTo("/question"), nil},
		{http.MethodPost, "/questions/{id}/reply", rewriteQuestionAction("/reply"), nil},
		{http.MethodPost, "/questions/{id}/reject", rewriteQuestionAction("/reject"), nil},
		{http.MethodGet, "/events", rewriteTo("/event"), nil},
		{http.MethodGet, "/agents/models", rewriteTo("/provider/capabilities"), nil},
		{http.MethodGet, "/agents/session-modes", rewriteTo("/agent"), nil},
		{http.MethodGet, "/agents/commands", rewriteTo("/command"), nil},
		{http.MethodGet, "/agents/mcp", rewriteTo("/mcp"), nil},
		{http.MethodGet, "/agents/lsp", rewriteTo("/lsp"), nil},
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

func renameJSONField(from, to string) func(io.ReadCloser) io.ReadCloser {
	return func(body io.ReadCloser) io.ReadCloser {
		pr, pw := io.Pipe()
		go func() {
			defer body.Close()
			defer pw.Close()
			buf, err := io.ReadAll(body)
			if err != nil {
				pw.CloseWithError(err)
				return
			}
			replaced := bytesReplaceKey(buf, from, to)
			pw.Write(replaced)
		}()
		return pr
	}
}

func bytesReplaceKey(data []byte, from, to string) []byte {
	fromKey := []byte(`"` + from + `"`)
	toKey := []byte(`"` + to + `"`)
	result := make([]byte, 0, len(data))
	i := 0
	for i < len(data) {
		if i+len(fromKey) <= len(data) && string(data[i:i+len(fromKey)]) == string(fromKey) {
			result = append(result, toKey...)
			i += len(fromKey)
			continue
		}
		result = append(result, data[i])
		i++
	}
	return result
}
