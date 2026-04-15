package localserver

import (
	"fmt"
	"net/http"
	"os/exec"
	"strings"
)

type diffFileEntry struct {
	Path      string `json:"path"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

type diffData struct {
	Directory string         `json:"directory"`
	Branch    string         `json:"branch"`
	Files     []diffFileEntry `json:"files"`
	Diff      string         `json:"diff,omitempty"`
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	directory := r.URL.Query().Get("directory")
	if directory == "" {
		directory = "."
	}

	absDir, _, err := resolvePath(r, directory)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	info, err := exec.Command("git", "-C", absDir, "rev-parse", "--is-inside-work-tree").Output()
	if err != nil || strings.TrimSpace(string(info)) != "true" {
		writeErr(w, http.StatusBadRequest, "NOT_GIT_REPO", fmt.Sprintf("not a git repository: %s", absDir))
		return
	}

	branch := gitBranch(absDir)

	staged := r.URL.Query().Get("staged") == "true"
	statOnly := r.URL.Query().Get("stat") == "true"
	filterPath := r.URL.Query().Get("path")

	files := parseDiffStat(absDir, staged, filterPath)

	var diff string
	if !statOnly {
		diff = runGitDiff(absDir, staged, filterPath)
	}

	writeOK(w, diffData{
		Directory: absDir,
		Branch:    branch,
		Files:     files,
		Diff:      diff,
	})
}

func parseDiffStat(dir string, staged bool, filterPath string) []diffFileEntry {
	args := []string{"-C", dir, "diff", "--numstat"}
	if staged {
		args = append(args, "--cached")
	}
	if filterPath != "" {
		args = append(args, "--", filterPath)
	}

	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil
	}

	var entries []diffFileEntry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}
		additions := parseNumstat(parts[0])
		deletions := parseNumstat(parts[1])
		path := parts[2]
		status := "modified"
		if additions < 0 {
			status = "deleted"
		} else if strings.HasPrefix(path, "a/") && strings.Contains(line, "=>") {
			status = "renamed"
		}

		entries = append(entries, diffFileEntry{
			Path:      path,
			Status:    status,
			Additions: additions,
			Deletions: deletions,
		})
	}

	if len(entries) == 0 && filterPath == "" {
		entries = []diffFileEntry{}
	}
	return entries
}

func runGitDiff(dir string, staged bool, filterPath string) string {
	args := []string{"-C", dir, "diff"}
	if staged {
		args = append(args, "--cached")
	}
	if filterPath != "" {
		args = append(args, "--", filterPath)
	}

	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}
	return string(out)
}

func (s *Server) handleConversationDiffDeprecated(w http.ResponseWriter, _ *http.Request) {
	writeErr(w, http.StatusNotImplemented, "DEPRECATED", "conversation diff is deprecated, use GET /runtime/diff instead")
}

func parseNumstat(s string) int {
	if s == "-" {
		return 0
	}
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}
