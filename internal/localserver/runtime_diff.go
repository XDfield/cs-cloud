package localserver

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
)

type diffFileEntry struct {
	Path      string `json:"path"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

type diffData struct {
	Directory     string          `json:"directory"`
	Branch        string          `json:"branch"`
	StagedFiles   []diffFileEntry `json:"stagedFiles"`
	UnstagedFiles []diffFileEntry `json:"unstagedFiles"`
}

type diffContentData struct {
	Diff   string `json:"diff"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	directory := r.URL.Query().Get("directory")
	if directory == "" {
		directory = "."
	}

	absDir, _, err := s.resolvePath(r, directory)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	filterPath := r.URL.Query().Get("path")

	var (
		branch        string
		stagedFiles   []diffFileEntry
		unstagedFiles []diffFileEntry
		mu            sync.Mutex
		wg            sync.WaitGroup
		notGit        bool
	)

	wg.Add(3)

	go func() {
		defer wg.Done()
		b, err := exec.Command("git", "-C", absDir, "rev-parse", "--abbrev-ref", "HEAD").Output()
		if err != nil {
			mu.Lock()
			notGit = true
			mu.Unlock()
			return
		}
		branch = strings.TrimSpace(string(b))
	}()

	go func() {
		defer wg.Done()
		entries, err := parseDiffStatErr(absDir, true, filterPath)
		if err != nil {
			mu.Lock()
			notGit = true
			mu.Unlock()
			return
		}
		stagedFiles = entries
	}()

	go func() {
		defer wg.Done()
		entries, err := parseDiffStatErr(absDir, false, filterPath)
		if err != nil {
			mu.Lock()
			notGit = true
			mu.Unlock()
			return
		}
		unstagedFiles = entries
	}()

	wg.Wait()

	if notGit {
		writeErr(w, http.StatusBadRequest, "NOT_GIT_REPO", fmt.Sprintf("not a git repository: %s", absDir))
		return
	}

	writeOK(w, diffData{
		Directory:     absDir,
		Branch:        branch,
		StagedFiles:   stagedFiles,
		UnstagedFiles: unstagedFiles,
	})
}

func parseDiffStatErr(dir string, staged bool, filterPath string) ([]diffFileEntry, error) {
	args := []string{"-C", dir, "diff", "--numstat"}
	if staged {
		args = append(args, "--cached")
	}
	if filterPath != "" {
		args = append(args, "--", filterPath)
	}

	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, err
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
	return entries, nil
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

func (s *Server) handleDiffContent(w http.ResponseWriter, r *http.Request) {
	absDir, _, err := s.resolvePath(r, ".")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	staged := r.URL.Query().Get("staged") == "true"
	filterPath := r.URL.Query().Get("path")

	beforeRef := "HEAD"
	if !staged {
		beforeRef = ""
	}

	writeOK(w, diffContentData{
		Diff:   runGitDiff(absDir, staged, filterPath),
		Before: gitShowFile(absDir, beforeRef, filterPath),
		After:  gitShowFile(absDir, "", filterPath),
	})
}

func gitShowFile(dir string, ref string, path string) string {
	if path == "" {
		return ""
	}
	var args []string
	if ref != "" {
		args = []string{"-C", dir, "show", ref + ":" + path}
	} else {
		absPath := dir + "/" + path
		data, err := os.ReadFile(absPath)
		if err != nil {
			return ""
		}
		return string(data)
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
