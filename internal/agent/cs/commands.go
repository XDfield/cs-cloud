package cs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"cs-cloud/internal/agent"
)

func (d *Driver) FetchCommands(endpoint string) ([]agent.SlashCommand, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(endpoint + "/command")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opencode returned status %d", resp.StatusCode)
	}

	var commands []agent.SlashCommand
	if err := json.NewDecoder(resp.Body).Decode(&commands); err != nil {
		return nil, err
	}
	return commands, nil
}
