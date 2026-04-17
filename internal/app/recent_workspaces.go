package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func (a *App) recentWorkspacesFile() string {
	return filepath.Join(a.rootDir, "recent_workspaces.json")
}

func (a *App) LoadRecentWorkspaces() ([]string, error) {
	if err := a.EnsureRootDir(); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(a.recentWorkspacesFile())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, err
	}
	return list, nil
}
