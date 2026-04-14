package agent

import (
	"context"
	"fmt"
	"os/exec"
)

type OpenCodeDriver struct {
	cliPath string
}

func NewOpenCodeDriver() *OpenCodeDriver {
	return &OpenCodeDriver{cliPath: OpenCodeCLIBinary}
}

func (d *OpenCodeDriver) Name() string { return "http" }

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
