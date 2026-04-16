package localserver

import (
	"bufio"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type fileEntry struct {
	Name     string    `json:"name"`
	Type     string    `json:"type"`
	Size     int64     `json:"size"`
	Modified time.Time `json:"modified"`
}

type fileListData struct {
	Path    string      `json:"path"`
	Entries []fileEntry `json:"entries"`
}

func (s *Server) handleFileList(w http.ResponseWriter, r *http.Request) {
	dirPath := r.URL.Query().Get("path")
	if dirPath == "" {
		dirPath = "."
	}

	recursive := r.URL.Query().Get("recursive") == "true"
	limit := 1000
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	absPath, _, err := s.resolvePath(r, dirPath)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("path not found: %s", absPath))
			return
		}
		writeErr(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	if !info.IsDir() {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", fmt.Sprintf("not a directory: %s", absPath))
		return
	}

	var entries []fileEntry
	if recursive {
		entries, err = walkDir(absPath, absPath, limit)
	} else {
		entries, err = readDir(absPath, limit)
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	writeOK(w, fileListData{
		Path:    absPath,
		Entries: entries,
	})
}

func readDir(dir string, limit int) ([]fileEntry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return entries[i].Name() < entries[j].Name()
	})

	result := make([]fileEntry, 0, min(len(entries), limit))
	for _, e := range entries {
		if len(result) >= limit {
			break
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		typ := "file"
		if e.IsDir() {
			typ = "directory"
		}
		result = append(result, fileEntry{
			Name:     e.Name(),
			Type:     typ,
			Size:     info.Size(),
			Modified: info.ModTime().UTC(),
		})
	}
	return result, nil
}

func walkDir(root, current string, limit int) ([]fileEntry, error) {
	var result []fileEntry
	err := filepath.WalkDir(current, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if len(result) >= limit {
			return fs.SkipAll
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		if rel == "." {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		typ := "file"
		if d.IsDir() {
			typ = "directory"
		}
		result = append(result, fileEntry{
			Name:     rel,
			Type:     typ,
			Size:     info.Size(),
			Modified: info.ModTime().UTC(),
		})
		return nil
	})
	return result, err
}

type fileContentData struct {
	Path        string `json:"path"`
	Content     string `json:"content"`
	Lines       int    `json:"lines"`
	Offset      int    `json:"offset"`
	TotalLines  int    `json:"total_lines"`
}

func (s *Server) handleFileContent(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", "path is required")
		return
	}

	offset := 1
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			offset = n
		}
	}

	limit := 2000
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	absPath, _, err := s.resolvePath(r, filePath)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeErr(w, http.StatusNotFound, "NOT_FOUND", fmt.Sprintf("file not found: %s", absPath))
			return
		}
		writeErr(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	if info.IsDir() {
		writeErr(w, http.StatusBadRequest, "BAD_REQUEST", fmt.Sprintf("not a file: %s", absPath))
		return
	}

	f, err := os.Open(absPath)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}
	defer f.Close()

	var allLines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		writeErr(w, http.StatusInternalServerError, "INTERNAL", err.Error())
		return
	}

	totalLines := len(allLines)
	start := offset - 1
	if start > totalLines {
		start = totalLines
	}
	end := start + limit
	if end > totalLines {
		end = totalLines
	}

	selected := allLines[start:end]
	content := strings.Join(selected, "\n")
	if len(selected) > 0 {
		content += "\n"
	}

	writeOK(w, fileContentData{
		Path:       absPath,
		Content:    content,
		Lines:      len(selected),
		Offset:     offset,
		TotalLines: totalLines,
	})
}