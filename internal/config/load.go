package config

import "cs-cloud/internal/platform"

func Load() (*Config, error) {
	return &Config{
		CloudBaseURL: platform.Getenv("COSTRICT_CLOUD_BASE_URL"),
		BaseURL:      platform.Getenv("COSTRICT_BASE_URL"),
	}, nil
}
