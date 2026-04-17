package localserver

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandleFindFilesMatchesBaseNameQuery(t *testing.T) {
	workspace := t.TempDir()
	match := filepath.Join(workspace, "packages")
	if err := os.MkdirAll(match, 0o755); err != nil {
		t.Fatalf("mkdir match dir: %v", err)
	}

	server := &Server{}
	req := httptest.NewRequest("GET", "/runtime/find/file?query=pack&dirs=true", nil)
	req.Header.Set(workspaceDirHeader, workspace)
	rec := httptest.NewRecorder()

	server.handleFindFiles(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var resp envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	body, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("marshal response data: %v", err)
	}
	var got []string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal response data: %v", err)
	}
	if len(got) != 1 || got[0] != match {
		t.Fatalf("unexpected results: %#v", got)
	}
}

func TestHandleFindFilesMatchesRelativePathQuery(t *testing.T) {
	workspace := t.TempDir()
	match := filepath.Join(workspace, "apps", "packages")
	if err := os.MkdirAll(match, 0o755); err != nil {
		t.Fatalf("mkdir match dir: %v", err)
	}

	other := filepath.Join(workspace, "packages")
	if err := os.MkdirAll(other, 0o755); err != nil {
		t.Fatalf("mkdir other dir: %v", err)
	}

	server := &Server{}
	req := httptest.NewRequest("GET", "/runtime/find/file?query=packages%2F&dirs=true", nil)
	req.Header.Set(workspaceDirHeader, workspace)
	rec := httptest.NewRecorder()

	server.handleFindFiles(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	var resp envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	body, err := json.Marshal(resp.Data)
	if err != nil {
		t.Fatalf("marshal response data: %v", err)
	}
	var got []string
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal response data: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("unexpected result count: %#v", got)
	}
	seen := map[string]bool{}
	for _, path := range got {
		seen[path] = true
	}
	if !seen[match] || !seen[other] {
		t.Fatalf("unexpected results: %#v", got)
	}
}
