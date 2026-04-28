package device

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"cs-cloud/internal/config"
	"cs-cloud/internal/platform"
	"cs-cloud/internal/provider"
	"cs-cloud/internal/version"
)

const cloudAPIPrefix = "cloud-api"

type Client struct {
	cfg *config.Config
}

func NewClient(cfg *config.Config) *Client {
	return &Client{cfg: cfg}
}

func GetCloudBaseURL(cfg *config.Config, credBaseURL string) string {
	if cfg == nil {
		cfg = &config.Config{}
	}
	raw := trimRight(firstNonEmpty(
		cfg.CloudBaseURL,
		credBaseURL,
		cfg.BaseURL,
		platform.Getenv("COSTRICT_CLOUD_BASE_URL"),
		platform.Getenv("COSTRICT_BASE_URL"),
		"https://zgsm.sangfor.com",
	), "/")
	if hasSuffix(raw, "/"+cloudAPIPrefix) {
		return raw
	}
	if cfg.CloudBaseURL != "" || platform.Getenv("COSTRICT_CLOUD_BASE_URL") != "" {
		return raw
	}
	return raw + "/" + cloudAPIPrefix
}

func GetCloudAPIURL(cfg *config.Config, p string, baseURL string) string {
	base := GetCloudBaseURL(cfg, baseURL)
	if hasPrefix(p, "/") {
		return base + p
	}
	return base + "/" + p
}

func (c *Client) CloudBaseURL() string {
	cred, err := provider.LoadCredentials()
	if err != nil || cred == nil {
		return GetCloudBaseURL(c.cfg, "")
	}
	return GetCloudBaseURL(c.cfg, cred.BaseURL)
}

func (c *Client) Register(ctx context.Context) (*DeviceInfo, error) {
	existing, err := LoadDevice()
	if err != nil {
		return nil, err
	}
	if existing != nil {
		resolved := GetCloudBaseURL(c.cfg, "")
		if resolved != existing.BaseURL {
			existing.BaseURL = resolved
			_ = SaveDevice(existing)
		}
		return existing, nil
	}

	creds, err := auth(ctx)
	if err != nil {
		return nil, err
	}

	base := GetCloudBaseURL(c.cfg, creds.BaseURL)
	deviceID := creds.MachineID

	info, err := enroll(ctx, creds, base, deviceID)
	if err != nil {
		if IsAuthError(err) && creds.RefreshToken != "" {
			creds, err = renew(ctx, creds)
			if err != nil {
				return nil, err
			}
			info, err = enroll(ctx, creds, base, deviceID)
		}
	}
	if err != nil {
		return nil, err
	}

	return info, nil
}

func auth(ctx context.Context) (*provider.Credentials, error) {
	creds, err := provider.LoadCredentials()
	if err != nil {
		return nil, err
	}
	if creds == nil || creds.AccessToken == "" {
		return nil, fmt.Errorf("not logged in: auth.json not found or access_token missing")
	}
	if creds.RefreshToken == "" || provider.IsTokenValid(creds.AccessToken, creds.RefreshToken, creds.ExpiryDate) {
		return creds, nil
	}
	return renew(ctx, creds)
}

func renew(ctx context.Context, creds *provider.Credentials) (*provider.Credentials, error) {
	if creds.RefreshToken == "" {
		return creds, nil
	}
	baseURL := provider.GetCoStrictBaseURL("", creds.BaseURL)
	result, err := provider.RefreshCoStrictToken(baseURL, creds.RefreshToken, creds.State)
	if err != nil {
		return nil, err
	}
	expiry := provider.ExtractExpiryFromJWT(result.AccessToken)
	fresh := &provider.Credentials{
		ID:           creds.ID,
		Name:         creds.Name,
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		State:        creds.State,
		MachineID:    creds.MachineID,
		BaseURL:      baseURL,
		ExpiryDate:   expiry,
		UpdatedAt:    time.Now().Format(time.RFC3339),
		ExpiredAt:    time.UnixMilli(expiry).Format(time.RFC3339),
	}
	if err := provider.SaveCredentials(fresh); err != nil {
		return nil, err
	}
	return fresh, nil
}

func enroll(ctx context.Context, creds *provider.Credentials, base, deviceID string) (*DeviceInfo, error) {
	reqBody := registerRequest{
		DeviceID:    deviceID,
		DisplayName: hostname(),
		Platform:    provider.JSPlatform(),
		Version:     version.Get(),
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}

	url := GetCloudAPIURL(nil, "/api/devices/register", base)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+creds.AccessToken)

	resp, err := platform.HTTPClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 409 {
		return handleConflict(resp, base)
	}

	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device registration failed: %d %s", resp.StatusCode, string(respBody))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device registration failed: %d %s", resp.StatusCode, string(respBody))
	}

	var out registerResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	info := &DeviceInfo{
		DeviceID:     out.Device.DeviceID,
		DeviceToken:  out.Token,
		RegisteredAt: time.Now().Format(time.RFC3339),
		BaseURL:      base,
	}
	if err := SaveDevice(info); err != nil {
		return nil, err
	}
	return info, nil
}

func handleConflict(resp *http.Response, base string) (*DeviceInfo, error) {
	var conflict conflictResponse
	if err := json.NewDecoder(resp.Body).Decode(&conflict); err != nil {
		return nil, fmt.Errorf("device already registered")
	}
	if conflict.Token != "" && conflict.Device != nil && conflict.Device.DeviceID != "" {
		info := &DeviceInfo{
			DeviceID:     conflict.Device.DeviceID,
			DeviceToken:  conflict.Token,
			RegisteredAt: time.Now().Format(time.RFC3339),
			BaseURL:      base,
		}
		if err := SaveDevice(info); err != nil {
			return nil, err
		}
		return info, nil
	}
	if conflict.Error != "" {
		return nil, fmt.Errorf("%s", conflict.Error)
	}
	return nil, fmt.Errorf("device already registered")
}

func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "401") || contains(msg, "403")
}

func IsMissingAuthError(err error) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), "not logged in")
}

func IsExpiredAuthError(err error) bool {
	if err == nil {
		return false
	}
	return contains(err.Error(), "invalid or expired")
}

func IsInvalidDeviceTokenError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "device token validation failed: 401") || contains(msg, "device token validation failed: 403")
}

func hostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "cs-cloud"
	}
	return h
}

func firstNonEmpty(vs ...string) string {
	for _, v := range vs {
		if v != "" {
			return v
		}
	}
	return ""
}

func trimRight(s, suffix string) string {
	for len(s) > 0 && hasSuffix(s, suffix) {
		s = s[:len(s)-len(suffix)]
	}
	return s
}

func hasSuffix(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && containsStr(s, sub)
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

type registerRequest struct {
	DeviceID    string `json:"deviceId"`
	DisplayName string `json:"displayName"`
	Platform    string `json:"platform"`
	Version     string `json:"version"`
}

type registerResponse struct {
	Device struct {
		DeviceID string `json:"deviceId"`
	} `json:"device"`
	Token string `json:"token"`
}

type conflictResponse struct {
	Device *struct {
		DeviceID string `json:"deviceId"`
	} `json:"device"`
	Token string `json:"token"`
	Error string `json:"error"`
}

// GetCloudBaseURL with nil-safe config
func GetCloudBaseURLNilSafe(cfg *config.Config, credBaseURL string) string {
	if cfg == nil {
		cfg = &config.Config{}
	}
	return GetCloudBaseURL(cfg, credBaseURL)
}

// Ensure Errors implements error interface check
var _ error = (*RegistrationError)(nil)

type RegistrationError struct {
	StatusCode int
	Message    string
}

func (e *RegistrationError) Error() string {
	return fmt.Sprintf("device registration failed: %d %s", e.StatusCode, e.Message)
}

// ClearDevice removes the local device.json file
func ClearDevice() error {
	p, err := DevicePath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
