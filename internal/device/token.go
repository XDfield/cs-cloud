package device

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"cs-cloud/internal/platform"
)

func (c *Client) RotateToken(ctx context.Context) error {
	dev, err := LoadDevice()
	if err != nil {
		return err
	}
	if dev == nil {
		return fmt.Errorf("device not registered")
	}

	url := GetCloudAPIURL(c.cfg, "/api/devices/"+dev.DeviceID+"/token/rotate", dev.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+dev.DeviceToken)

	resp, err := platform.HTTPClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("token rotation failed: %d %s", resp.StatusCode, string(body))
	}

	var data struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}

	dev.DeviceToken = data.Token
	if err := SaveDevice(dev); err != nil {
		return err
	}
	return nil
}

func (c *Client) Heartbeat() error {
	return nil
}

func (c *Client) SetOnline() error {
	return nil
}
