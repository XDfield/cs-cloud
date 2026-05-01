package localserver

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

func readJSON(r *http.Request, v any) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return fmt.Errorf("read body: %w", err)
	}
	defer r.Body.Close()
	if len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, v)
}

func decodeQueryParam(v string) string {
	if decoded, err := url.QueryUnescape(v); err == nil {
		return decoded
	}
	return v
}

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x", b)
}

const workspaceDirHeader = "X-Workspace-Directory"

func getWorkspaceDir(r *http.Request) string {
	v := r.Header.Get(workspaceDirHeader)
	if decoded, err := url.PathUnescape(v); err == nil {
		return decoded
	}
	return v
}

func (s *Server) resolvePath(r *http.Request, relPath string) (absPath string, workspace string, err error) {
	workspace = getWorkspaceDir(r)
	if workspace == "" {
		workspace, _ = os.Getwd()
	}
	workspace = filepath.Clean(workspace)
	if !filepath.IsAbs(workspace) {
		workspace, err = filepath.Abs(workspace)
		if err != nil {
			return "", "", fmt.Errorf("invalid workspace directory: %w", err)
		}
	}

	if filepath.IsAbs(relPath) && s.runtimeCfg.AllowAbsolutePaths {
		absPath = filepath.Clean(relPath)
		return absPath, workspace, nil
	}

	absPath = filepath.Clean(filepath.Join(workspace, relPath))
	if !strings.HasPrefix(absPath, workspace+string(filepath.Separator)) && absPath != workspace {
		return "", "", fmt.Errorf("path escapes workspace directory")
	}
	return absPath, workspace, nil
}

func resolvePath(r *http.Request, relPath string) (absPath string, workspace string, err error) {
	workspace = getWorkspaceDir(r)
	if workspace == "" {
		workspace, _ = os.Getwd()
	}
	workspace = filepath.Clean(workspace)
	if !filepath.IsAbs(workspace) {
		workspace, err = filepath.Abs(workspace)
		if err != nil {
			return "", "", fmt.Errorf("invalid workspace directory: %w", err)
		}
	}

	absPath = filepath.Clean(filepath.Join(workspace, relPath))
	if !strings.HasPrefix(absPath, workspace+string(filepath.Separator)) && absPath != workspace {
		return "", "", fmt.Errorf("path escapes workspace directory")
	}
	return absPath, workspace, nil
}
