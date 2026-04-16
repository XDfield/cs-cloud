package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"time"

	"cs-cloud/internal/version"
)

type CheckResult struct {
	CanUpdate      bool   `json:"can_update"`
	Version        string `json:"version"`
	Changelog      string `json:"changelog,omitempty"`
	DownloadURL    string `json:"download_url,omitempty"`
	SHA256         string `json:"sha256,omitempty"`
	Force          bool   `json:"force,omitempty"`
	MinClientVer   string `json:"min_client_version,omitempty"`
	ReleaseDate    string `json:"release_date,omitempty"`
	BinarySize     int64  `json:"size,omitempty"`
}

type Checker struct {
	baseURL    string
	httpClient *http.Client
}

func NewChecker(baseURL string) *Checker {
	return &Checker{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Checker) Check(ctx context.Context) (*CheckResult, error) {
	platform := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base url: %w", err)
	}
	u.Path, err = url.JoinPath(u.Path, "/api/updates/check")
	if err != nil {
		return nil, fmt.Errorf("build check url: %w", err)
	}
	q := u.Query()
	q.Set("platform", platform)
	q.Set("version", version.Get())
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", version.UserAgent())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("update check failed (status %d): %s", resp.StatusCode, string(body))
	}

	var result CheckResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode check response: %w", err)
	}
	return &result, nil
}
