package localserver

import (
	"net/http"

	"cs-cloud/internal/config"
)

type runtimeConfigData struct {
	AllowAbsolutePaths bool     `json:"allow_absolute_paths"`
	MaxListDepth       int      `json:"max_list_depth"`
	AllowedOperations  []string `json:"allowed_operations"`
	BlacklistCount     int      `json:"blacklist_count"`
	WhitelistEnabled   bool     `json:"whitelist_enabled"`
}

func (s *Server) handleRuntimeConfig(w http.ResponseWriter, _ *http.Request) {
	cfg := s.runtimeCfg
	writeOK(w, runtimeConfigData{
		AllowAbsolutePaths: cfg.AllowAbsolutePaths,
		MaxListDepth:       cfg.MaxListDepth,
		AllowedOperations:  cfg.AllowedOperations,
		BlacklistCount:     cfg.BlacklistCount,
		WhitelistEnabled:   cfg.WhitelistEnabled,
	})
}

func defaultRuntimeConfig() config.RuntimeConfig {
	return config.RuntimeConfig{
		AllowAbsolutePaths: true,
		MaxListDepth:       0,
		AllowedOperations:  []string{"list", "read", "search"},
		BlacklistCount:     0,
		WhitelistEnabled:   false,
	}
}
