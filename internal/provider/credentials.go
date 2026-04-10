package provider

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"cs-cloud/internal/platform"
)

func CredentialsPath() (string, error) {
	return filepath.Join(platform.CoStrictShareDir(), "auth.json"), nil
}

func LoadCredentials() (*Credentials, error) {
	p, err := CredentialsPath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var cred Credentials
	if err := json.Unmarshal(b, &cred); err != nil {
		return nil, nil
	}
	if cred.AccessToken == "" || cred.BaseURL == "" {
		return nil, nil
	}
	return &cred, nil
}

func SaveCredentials(cred *Credentials) error {
	p, err := CredentialsPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create credentials dir: %w", err)
	}
	b, err := json.MarshalIndent(cred, "", "  ")
	if err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}
	return nil
}

func DeleteCredentials() error {
	p, err := CredentialsPath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func GetCoStrictBaseURL(providerAPI string, credBaseURL string) string {
	envURL := platform.Getenv("COSTRICT_BASE_URL")
	defaultURL := "https://zgsm.sangfor.com"
	raw := firstNonEmpty(envURL, providerAPI, credBaseURL, defaultURL)
	raw = trimSuffix(raw, "/chat-rag/api/v1")
	raw = trimSuffix(raw, "/")
	return raw
}

func BuildOAuthParams(includeMachineCode bool, machineID string, state string) []string {
	var params []string
	if includeMachineCode {
		if machineID == "" {
			panic("machineID is required when includeMachineCode is true")
		}
		params = append(params, "machine_code="+url.QueryEscape(machineID))
	}
	if state != "" {
		params = append(params, "state="+url.QueryEscape(state))
	}
	version := "costrict-cli-dev"
	params = append(params,
		"provider=casdoor",
		"plugin_version="+url.QueryEscape(version),
		"vscode_version="+url.QueryEscape(version),
		"uri_scheme=costrict-cli",
	)
	return params
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

func trimSuffix(s, suffix string) string {
	for len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix {
		s = s[:len(s)-len(suffix)]
	}
	return s
}
