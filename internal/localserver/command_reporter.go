package localserver

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"cs-cloud/internal/device"
	"cs-cloud/internal/logger"
	"cs-cloud/internal/platform"
)

type CommandReporter struct {
	deviceClient *device.Client
}

func NewCommandReporter() *CommandReporter {
	return &CommandReporter{}
}

func (r *CommandReporter) Report(commandID string, result *commandStatusResponse) {
	dev, err := device.LoadDevice()
	if err != nil || dev == nil {
		logger.Warn("[reporter] cannot load device for reporting: %v", err)
		return
	}

	body, err := json.Marshal(result)
	if err != nil {
		logger.Error("[reporter] marshal result failed: %v", err)
		return
	}

	url := device.GetCloudAPIURL(nil, "/api/devices/"+dev.DeviceID+"/commands/"+commandID+"/result", dev.BaseURL)

	for attempt := 0; attempt < 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
		if reqErr != nil {
			cancel()
			continue
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+dev.DeviceToken)

		resp, doErr := platform.HTTPClient().Do(req)
		cancel()
		if doErr != nil {
			logger.Warn("[reporter] report attempt %d failed: %v", attempt+1, doErr)
			time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
			continue
		}
		resp.Body.Close()

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			logger.Info("[reporter] command %s result reported successfully", commandID)
			return
		}
		logger.Warn("[reporter] report attempt %d returned %d", attempt+1, resp.StatusCode)
	}

	logger.Error("[reporter] failed to report command %s after 3 attempts", commandID)
}
