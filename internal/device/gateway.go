package device

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"cs-cloud/internal/platform"
	"cs-cloud/internal/version"
)

func ValidateDeviceToken(ctx context.Context, device *DeviceInfo) error {
	reqBody := map[string]any{
		"deviceID": device.DeviceID,
		"version":  version.Get(),
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := GetCloudAPIURL(nil, "/cloud/device/gateway-assign", device.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+device.DeviceToken)

	resp, err := platform.HTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("device token validation failed: %d %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func AssignGateway(ctx context.Context, device *DeviceInfo) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	reqBody := map[string]any{
		"deviceID": device.DeviceID,
		"version":  version.Get(),
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	url := GetCloudAPIURL(nil, "/cloud/device/gateway-assign", device.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+device.DeviceToken)

	resp, err := platform.HTTPClient().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("gateway-assign failed: %d %s", resp.StatusCode, string(respBody))
	}

	var data struct {
		GatewayURL string `json:"gatewayURL"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	return data.GatewayURL, nil
}
