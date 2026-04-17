package localserver

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func (s *Server) handleFindFiles(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	dirs := r.URL.Query().Get("dirs")
	limitStr := r.URL.Query().Get("limit")

	limit := 10
	if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 200 {
		limit = n
	}

	includeDirs := dirs != "false"

	dir := r.URL.Query().Get("directory")
	if dir == "" {
		dir = "."
	}

	absDir, _, err := s.resolvePath(r, dir)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	info, err := os.Stat(absDir)
	if err != nil || !info.IsDir() {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", fmt.Sprintf("directory not found: %s", absDir))
		return
	}

	var results []string
	query = filepath.ToSlash(strings.ToLower(query))
	matchRelativePath := strings.Contains(query, "/")

	err = filepath.WalkDir(absDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		rel, err := filepath.Rel(absDir, path)
		if err != nil || rel == "." {
			return nil
		}

		if shouldSkip(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if len(results) >= limit {
			return filepath.SkipDir
		}

		candidate := strings.ToLower(filepath.Base(path))
		if matchRelativePath {
			candidate = filepath.ToSlash(strings.ToLower(rel))
		}
		if d.IsDir() {
			candidate += "/"
		}

		if query != "" && !strings.Contains(candidate, query) {
			return nil
		}

		if !includeDirs && d.IsDir() {
			return nil
		}

		results = append(results, path)
		return nil
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	writeOK(w, results)
}

func shouldSkip(rel string) bool {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) == 0 {
		return false
	}
	top := parts[0]
	skipDirs := map[string]bool{
		"node_modules": true,
		".git":         true,
		".svn":         true,
		".hg":          true,
		"__pycache__":  true,
		".next":        true,
		".nuxt":        true,
		"dist":         true,
		"build":        true,
		"target":       true,
		".cache":       true,
		".tox":         true,
		".venv":        true,
		"venv":         true,
		".idea":        true,
	}
	return skipDirs[top]
}
