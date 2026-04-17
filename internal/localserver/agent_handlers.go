package localserver

import (
	"context"
	"net/http"
	"time"
)

type agentHealthProbeResult struct {
	Available bool   `json:"available"`
	LatencyMs int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	detected, err := s.manager.DetectAgents(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	agents := make([]map[string]any, 0, len(detected))
	for _, da := range detected {
		info := map[string]any{
			"id":        da.Backend,
			"name":      da.Name,
			"driver":    da.Driver,
			"available": da.Available,
		}
		if extra, ok := da.Extra.(map[string]any); ok {
			if ep, ok := extra["endpoint"].(string); ok {
				info["endpoint"] = ep
			}
		}
		agents = append(agents, info)
	}

	writeOK(w, map[string]any{"agents": agents})
}

func (s *Server) handleAgentHealth(w http.ResponseWriter, r *http.Request) {
	agents := s.manager.ListAgents()
	if len(agents) == 0 {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "no agent running")
		return
	}

	results := make([]map[string]any, 0, len(agents))
	for _, a := range agents {
		probe := probeAgentHealth(r.Context(), a)
		results = append(results, map[string]any{
			"id":        a.ID(),
			"backend":   a.Backend(),
			"driver":    a.Driver(),
			"state":     a.State().String(),
			"available": probe.Available,
			"latency_ms": probe.LatencyMs,
			"error":     probe.Error,
		})
	}
	writeOK(w, map[string]any{"agents": results})
}

func probeAgentHealth(parent context.Context, a interface{}) agentHealthProbeResult {
	endpointAgent, ok := a.(interface{ Endpoint() string })
	if !ok || endpointAgent.Endpoint() == "" {
		return agentHealthProbeResult{Available: false, Error: "agent endpoint unavailable"}
	}

	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpointAgent.Endpoint()+"/global/health", nil)
	if err != nil {
		return agentHealthProbeResult{Available: false, Error: err.Error()}
	}

	start := time.Now()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return agentHealthProbeResult{Available: false, Error: err.Error()}
	}
	defer resp.Body.Close()

	latencyMs := time.Since(start).Milliseconds()
	if resp.StatusCode != http.StatusOK {
		return agentHealthProbeResult{
			Available: false,
			LatencyMs: latencyMs,
			Error:     resp.Status,
		}
	}

	return agentHealthProbeResult{Available: true, LatencyMs: latencyMs}
}
