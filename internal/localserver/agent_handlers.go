package localserver

import (
	"net/http"
)

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
	ctx := r.Context()

	result, err := s.manager.HealthCheck(ctx, "opencode")
	if err != nil {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", err.Error())
		return
	}

	if !result.Available {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", result.Error)
		return
	}

	writeOK(w, result)
}
