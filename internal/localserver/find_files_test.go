package localserver

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cs-cloud/internal/config"
	"cs-cloud/internal/runtime"
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

func TestHandleFindFilesSkipsNestedIgnoredDirectories(t *testing.T) {
	workspace := t.TempDir()
	allowed := filepath.Join(workspace, "packages", "app-ai")
	ignored := filepath.Join(workspace, "packages", "nested", "node_modules", "app-ai")
	if err := os.MkdirAll(allowed, 0o755); err != nil {
		t.Fatalf("mkdir allowed dir: %v", err)
	}
	if err := os.MkdirAll(ignored, 0o755); err != nil {
		t.Fatalf("mkdir ignored dir: %v", err)
	}

	server := &Server{}
	req := httptest.NewRequest("GET", "/runtime/find/file?query=app-ai&dirs=true&limit=10", nil)
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
	if len(got) != 1 || got[0] != allowed {
		t.Fatalf("unexpected results: %#v", got)
	}
}

func TestHandleFindFilesStopsAfterLimit(t *testing.T) {
	workspace := t.TempDir()
	allowedRoot := filepath.Join(workspace, "match-root")
	deepRoot := filepath.Join(workspace, "deep")
	if err := os.MkdirAll(allowedRoot, 0o755); err != nil {
		t.Fatalf("mkdir allowed root: %v", err)
	}
	if err := os.MkdirAll(deepRoot, 0o755); err != nil {
		t.Fatalf("mkdir deep root: %v", err)
	}

	for i := 0; i < 5; i++ {
		if err := os.MkdirAll(filepath.Join(allowedRoot, "match-"+strings.Repeat("a", i+1)), 0o755); err != nil {
			t.Fatalf("mkdir allowed match dir: %v", err)
		}
	}
	for i := 0; i < 50; i++ {
		if err := os.MkdirAll(filepath.Join(deepRoot, "branch", "sub", "match-"+strings.Repeat("b", i+1)), 0o755); err != nil {
			t.Fatalf("mkdir deep match dir: %v", err)
		}
	}

	server := &Server{}
	req := httptest.NewRequest("GET", "/runtime/find/file?query=match&dirs=true&limit=3", nil)
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
	if len(got) != 3 {
		t.Fatalf("expected 3 results, got %#v", got)
	}
}

func TestFindFilesIndexCacheReusedWithinTTL(t *testing.T) {
	workspace := t.TempDir()
	first := filepath.Join(workspace, "app-ai")
	if err := os.MkdirAll(first, 0o755); err != nil {
		t.Fatalf("mkdir first dir: %v", err)
	}

	server := &Server{}
	idx1, err := server.findFilesIndex(workspace)
	if err != nil {
		t.Fatalf("first build failed: %v", err)
	}

	second := filepath.Join(workspace, "app-ai-2")
	if err := os.MkdirAll(second, 0o755); err != nil {
		t.Fatalf("mkdir second dir: %v", err)
	}

	idx2, err := server.findFilesIndex(workspace)
	if err != nil {
		t.Fatalf("second build failed: %v", err)
	}
	if idx1 != idx2 {
		t.Fatalf("expected cache reuse, got different index pointers")
	}

	results := searchIndexedPaths(idx2, workspace, "app-ai-2", true, 10)
	if len(results) != 0 {
		t.Fatalf("expected cached index to exclude fresh path, got %#v", results)
	}
}

func TestFindFilesIndexCacheRefreshAfterTTL(t *testing.T) {
	workspace := t.TempDir()
	first := filepath.Join(workspace, "app-ai")
	if err := os.MkdirAll(first, 0o755); err != nil {
		t.Fatalf("mkdir first dir: %v", err)
	}

	server := &Server{}
	idx1, err := server.findFilesIndex(workspace)
	if err != nil {
		t.Fatalf("first build failed: %v", err)
	}

	server.findFilesMu.Lock()
	server.findFilesCache[workspace].builtAt = time.Now().Add(-findFilesCacheTTL - time.Second)
	server.findFilesMu.Unlock()

	second := filepath.Join(workspace, "app-ai-2")
	if err := os.MkdirAll(second, 0o755); err != nil {
		t.Fatalf("mkdir second dir: %v", err)
	}

	idx2, err := server.findFilesIndex(workspace)
	if err != nil {
		t.Fatalf("second build failed: %v", err)
	}
	if idx1 == idx2 {
		t.Fatalf("expected rebuilt index after TTL expiry")
	}

	results := searchIndexedPaths(idx2, workspace, "app-ai-2", true, 10)
	if len(results) != 1 || results[0] != second {
		t.Fatalf("expected refreshed index to include fresh path, got %#v", results)
	}
}

func TestSearchIndexedPathsIncludesDirsAndFiles(t *testing.T) {
	idx := &fileSearchIndex{
		files: []string{"src/app-ai.ts"},
		dirs:  []string{"packages/app-ai/"},
	}

	workspace := t.TempDir()
	got := searchIndexedPaths(idx, workspace, "app-ai", true, 10)
	if len(got) != 2 {
		t.Fatalf("unexpected result count: %#v", got)
	}
	seen := map[string]bool{}
	for _, item := range got {
		seen[item] = true
	}
	if !seen[filepath.Join(workspace, "src", "app-ai.ts")] || !seen[filepath.Join(workspace, "packages", "app-ai")] {
		t.Fatalf("expected file and dir results, got %#v", got)
	}
}

func TestSearchIndexedPathsRanksBasenamePrefixBeforePathContains(t *testing.T) {
	idx := &fileSearchIndex{
		files: []string{
			"nested/some-app-ai-notes.txt",
			"app-ai.ts",
			"src/app-ai-helper.ts",
		},
	}

	workspace := t.TempDir()
	got := searchIndexedPaths(idx, workspace, "app-ai", false, 10)

	if len(got) < 3 {
		t.Fatalf("unexpected result count: %#v", got)
	}
	if got[0] != filepath.Join(workspace, "app-ai.ts") {
		t.Fatalf("expected exact basename match first, got %#v", got)
	}
	if got[1] != filepath.Join(workspace, "src", "app-ai-helper.ts") {
		t.Fatalf("expected basename prefix match second, got %#v", got)
	}
}

func TestSearchIndexedPathsPrefersShallowerPathOnTie(t *testing.T) {
	idx := &fileSearchIndex{
		files: []string{
			"deep/nested/app-ai.txt",
			"top/app-ai.txt",
		},
	}

	workspace := t.TempDir()
	got := searchIndexedPaths(idx, workspace, "app-ai", false, 10)

	if len(got) < 2 {
		t.Fatalf("unexpected result count: %#v", got)
	}
	if got[0] != filepath.Join(workspace, "top", "app-ai.txt") {
		t.Fatalf("expected shallower path first, got %#v", got)
	}
}

func TestInvalidateFindFilesCacheByWorkspace(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "app-ai"), 0o755); err != nil {
		t.Fatalf("mkdir workspace dir: %v", err)
	}

	server := &Server{}
	idx, err := server.findFilesIndex(workspace)
	if err != nil {
		t.Fatalf("build cache failed: %v", err)
	}
	if idx == nil {
		t.Fatal("expected index")
	}

	server.invalidateFindFilesCache(workspace)

	server.findFilesMu.Lock()
	_, ok := server.findFilesCache[workspace]
	server.findFilesMu.Unlock()
	if ok {
		t.Fatalf("expected workspace cache entry to be removed")
	}
}

func TestHandleInstanceDisposeInvalidatesWorkspaceCache(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "app-ai"), 0o755); err != nil {
		t.Fatalf("mkdir workspace dir: %v", err)
	}

	server := &Server{}
	server.cfg = &config.Config{}
	server.manager = runtime.NewAgentManager(runtime.NewEventBus())
	_, err := server.findFilesIndex(workspace)
	if err != nil {
		t.Fatalf("build cache failed: %v", err)
	}

	req := httptest.NewRequest("POST", "/runtime/dispose", nil)
	req.Header.Set(workspaceDirHeader, workspace)
	rec := httptest.NewRecorder()

	server.handleInstanceDispose(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	server.findFilesMu.Lock()
	_, ok := server.findFilesCache[workspace]
	server.findFilesMu.Unlock()
	if ok {
		t.Fatalf("expected workspace cache entry to be removed after dispose")
	}
}
