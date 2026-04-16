package config

type Config struct {
	CloudBaseURL string        `json:"cloud_base_url"`
	BaseURL      string        `json:"base_url"`
	DefaultShell string        `json:"default_shell"`
	AgentCLIPath string        `json:"agent_cli_path"`
	Runtime      RuntimeConfig `json:"runtime"`
}

type RuntimeConfig struct {
	AllowAbsolutePaths bool     `json:"allow_absolute_paths"`
	MaxListDepth       int      `json:"max_list_depth"`
	AllowedOperations  []string `json:"allowed_operations"`
	BlacklistCount     int      `json:"blacklist_count"`
	WhitelistEnabled   bool     `json:"whitelist_enabled"`
}
