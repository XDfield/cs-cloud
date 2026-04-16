package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"cs-cloud/internal/platform"
)

func Load() (*Config, error) {
	cfg := &Config{
		CloudBaseURL: platform.Getenv("CLOUD_BASE_URL"),
		BaseURL:      platform.Getenv("COSTRICT_BASE_URL"),
		DefaultShell: platform.Getenv("CS_CLOUD_SHELL"),
		AgentCLIPath: platform.Getenv("CS_CLOUD_AGENT_CLI"),
	}

	if cfg.CloudBaseURL == "" {
		cfg.CloudBaseURL = platform.Getenv("COSTRICT_CLOUD_BASE_URL")
	}

	if envJSON := platform.Getenv("CS_CLOUD_AGENT_ENV"); envJSON != "" {
		var env map[string]string
		if err := json.Unmarshal([]byte(envJSON), &env); err == nil {
			cfg.AgentEnv = env
		}
	}

	if p, err := configFilePath(); err == nil {
		if b, err := os.ReadFile(p); err == nil {
			var fileCfg Config
			if err := json.Unmarshal(b, &fileCfg); err == nil {
				if cfg.CloudBaseURL == "" {
					cfg.CloudBaseURL = fileCfg.CloudBaseURL
				}
				if cfg.BaseURL == "" {
					cfg.BaseURL = fileCfg.BaseURL
				}
				if cfg.DefaultShell == "" {
					cfg.DefaultShell = fileCfg.DefaultShell
				}
				if cfg.AgentCLIPath == "" {
					cfg.AgentCLIPath = fileCfg.AgentCLIPath
				}
				if cfg.AgentEnv == nil && fileCfg.AgentEnv != nil {
					cfg.AgentEnv = fileCfg.AgentEnv
				}
			}
		}
	}

	return cfg, nil
}

func configFilePath() (string, error) {
	return filepath.Join(platform.AppDir(), "config.json"), nil
}
