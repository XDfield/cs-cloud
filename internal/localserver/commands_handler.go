package localserver

import (
	"net/http"

	"cs-cloud/internal/agent"
	"cs-cloud/internal/logger"
)

func (s *Server) handleCommands(w http.ResponseWriter, r *http.Request) {
	scopes := agent.ParseIncludeScopes(r.URL.Query().Get("include"))

	var opencodeCmds []agent.SlashCommand

	endpoint := s.manager.Endpoint()
	if endpoint != "" {
		backend := s.manager.DefaultBackend()
		d, err := s.manager.ResolveDriver(backend)
		if err != nil {
			logger.Warn("no driver for backend %s, falling back to builtin only: %v", backend, err)
		} else {
			cmds, err := d.FetchCommands(endpoint)
			if err != nil {
				logger.Warn("failed to fetch opencode commands, falling back to builtin only: %v", err)
			} else {
				opencodeCmds = cmds
			}
		}
	} else {
		logger.Warn("no agent endpoint available, returning builtin commands only")
	}

	manifest, err := agent.BuildManifest(scopes, opencodeCmds)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, manifest)
}
