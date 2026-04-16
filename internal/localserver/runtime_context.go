package localserver

import (
	"net/http"
	"os"
	"os/exec"
	"strings"

	"cs-cloud/internal/logger"
)

type pathData struct {
	Home      string `json:"home"`
	Directory string `json:"directory"`
}

func (s *Server) handlePath(w http.ResponseWriter, r *http.Request) {
	home, _ := os.UserHomeDir()

	directory := r.URL.Query().Get("directory")
	if directory == "" {
		directory = "."
	}

	absDir, _, err := s.resolvePath(r, directory)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	writeOK(w, pathData{
		Home:      home,
		Directory: absDir,
	})
}

type vcsData struct {
	Branch string `json:"branch"`
}

func (s *Server) handleVcs(w http.ResponseWriter, r *http.Request) {
	directory := r.URL.Query().Get("directory")
	if directory == "" {
		directory = "."
	}

	absDir, _, err := s.resolvePath(r, directory)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	branch := gitBranch(absDir)
	writeOK(w, vcsData{Branch: branch})
}

func (s *Server) handleInstanceDispose(w http.ResponseWriter, r *http.Request) {
	s.manager.KillAll()

	ctx := r.Context()
	if err := s.manager.InitDefaultAgent(ctx, ""); err != nil {
		logger.Error("failed to restart opencode agent: %v", err)
	}

	writeOK(w, map[string]any{"disposed": true})
}

func gitBranch(dir string) string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
