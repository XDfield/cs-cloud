package localserver

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"cs-cloud/internal/logger"
)

type pathData struct {
	Home      string `json:"home" example:"/home/user"`
	Directory string `json:"directory" example:"/home/user/project"`
}

// @Summary      Get path information
// @Description  Returns the user home directory and the resolved absolute workspace directory.
// @Tags         Runtime
// @Produce      json
// @Param        directory  query  string  false  "Target directory (relative to workspace root)"  default(.)
// @Success      200  {object}  envelope{data=pathData}
// @Failure      400  {object}  envelope
// @Router       /runtime/path [get]
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
	Branch string `json:"branch" example:"main"`
}

// @Summary      Get Git branch info
// @Description  Returns the current Git branch for the specified directory. Returns empty branch if not a git repository.
// @Tags         Runtime
// @Produce      json
// @Param        directory  query  string  false  "Target directory (relative to workspace root)"  default(.)
// @Success      200  {object}  envelope{data=vcsData}
// @Failure      400  {object}  envelope
// @Router       /runtime/vcs [get]
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

// @Summary      Kill all agents and reinitialize
// @Description  Terminates all running agent instances and restarts the default agent.
// @Tags         Runtime
// @Produce      json
// @Success      200  {object}  envelope{data=map[string]bool}
// @Router       /runtime/dispose [post]
func (s *Server) handleInstanceDispose(w http.ResponseWriter, r *http.Request) {
	workspace := r.Header.Get(workspaceDirHeader)
	if workspace != "" {
		if abs, err := filepath.Abs(filepath.Clean(workspace)); err == nil {
			s.invalidateFindFilesCache(abs)
		} else {
			s.invalidateFindFilesCache(filepath.Clean(workspace))
		}
	} else {
		s.invalidateFindFilesCache("")
	}

	s.manager.KillAll()

	ctx := r.Context()
	if err := s.manager.InitDefaultAgent(ctx, s.cfg.DefaultAgent, "", s.cfg.AgentWorkspace, nil); err != nil {
		logger.Error("failed to restart agent: %v", err)
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
