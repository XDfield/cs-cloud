package localserver

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const recentMax = 12

func (s *Server) recentFile() string {
	if s.rootDir == "" {
		return ""
	}
	return filepath.Join(s.rootDir, "recent_workspaces.json")
}

func (s *Server) rememberWorkspace(dir string) {
	if dir == "" || s.rootDir == "" {
		return
	}

	s.recentMu.Lock()
	defer s.recentMu.Unlock()

	list := s.loadRecentLocked()
	next := []string{dir}
	for _, item := range list {
		if item == "" || item == dir {
			continue
		}
		next = append(next, item)
		if len(next) >= recentMax {
			break
		}
	}

	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.recentFile(), data, 0o644)
}

func (s *Server) loadRecentLocked() []string {
	file := s.recentFile()
	if file == "" {
		return nil
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	var list []string
	if json.Unmarshal(data, &list) != nil {
		return nil
	}
	return list
}
