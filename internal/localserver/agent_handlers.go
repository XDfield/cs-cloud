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
	agents := s.manager.ListAgents()
	if len(agents) == 0 {
		writeErr(w, http.StatusServiceUnavailable, "UNAVAILABLE", "no agent running")
		return
	}

	results := make([]map[string]any, 0, len(agents))
	for _, a := range agents {
		results = append(results, map[string]any{
			"id":        a.ID(),
			"backend":   a.Backend(),
			"driver":    a.Driver(),
			"state":     a.State().String(),
			"available": a.State() >= 2,
		})
	}
	writeOK(w, map[string]any{"agents": results})
}
