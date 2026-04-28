package localserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cs-cloud/internal/logger"
)

func (s *Server) handleCommands(w http.ResponseWriter, r *http.Request) {
	scopes := parseIncludeScopes(r.URL.Query().Get("include"))

	var opencodeCmds []Command

	endpoint := s.manager.Endpoint()
	if endpoint != "" {
		cmds, err := fetchOpenCodeCommands(endpoint)
		if err != nil {
			logger.Warn("failed to fetch opencode commands, falling back to builtin only: %v", err)
		} else {
			opencodeCmds = cmds
		}
	} else {
		logger.Warn("no agent endpoint available, returning builtin commands only")
	}

	manifest, err := BuildManifest(scopes, opencodeCmds)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, manifest)
}

func fetchOpenCodeCommands(endpoint string) ([]Command, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(endpoint + "/command")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opencode returned status %d", resp.StatusCode)
	}

	var commands []Command
	if err := json.NewDecoder(resp.Body).Decode(&commands); err != nil {
		return nil, err
	}
	return commands, nil
}
