package config

type Config struct {
	CloudBaseURL string `json:"cloud_base_url"`
	BaseURL      string `json:"base_url"`
	DefaultShell string `json:"default_shell"`
	AgentCLIPath string `json:"agent_cli_path"`
}
