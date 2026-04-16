package config

func Default() *Config {
	return &Config{
		Runtime: RuntimeConfig{
			AllowAbsolutePaths: true,
			MaxListDepth:       0,
			AllowedOperations:  []string{"list", "read", "search"},
			BlacklistCount:     0,
			WhitelistEnabled:   false,
		},
	}
}
