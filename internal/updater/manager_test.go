package updater

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEndToEndUpgradeFlow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping end-to-end replace test on windows (file locking)")
	}

	binaryContent := []byte("#!/bin/sh\necho v2.0.0\n")
	expectedSHA := fmt.Sprintf("%x", sha256.Sum256(binaryContent))

	checkResp := map[string]any{
		"can_update":    true,
		"version":      "v2.0.0",
		"download_url": "", // filled below
		"sha256":       expectedSHA,
		"changelog":    "test release",
	}
	downloadSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(binaryContent)
	}))
	defer downloadSrv.Close()
	checkResp["download_url"] = downloadSrv.URL

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/updates/check" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(checkResp)
			return
		}
		http.NotFound(w, r)
	}))
	defer apiSrv.Close()

	dir := t.TempDir()
	currentExe := filepath.Join(dir, "cs-cloud")
	oldContent := []byte("#!/bin/sh\necho v1.0.0\n")
	os.WriteFile(currentExe, oldContent, 0o755)

	upgradesDir := filepath.Join(dir, "upgrades")
	r := NewReplacer(upgradesDir)

	mgr := &Manager{
		checker:    NewChecker(apiSrv.URL),
		downloader: NewDownloader(filepath.Join(upgradesDir, "tmp")),
		verifier:   NewVerifier(),
		replacer:   r,
		policy:     PolicyAuto,
		interval:   6 * time.Hour,
		RestartCh:  make(chan struct{}, 1),
	}

	result, err := mgr.CheckNow(context.Background())
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if !result.CanUpdate {
		t.Fatal("expected update available")
	}
	if result.SHA256 != expectedSHA {
		t.Fatalf("sha256 mismatch: got %s", result.SHA256)
	}

	newBinary, err := mgr.downloadAndVerify(context.Background(), result)
	if err != nil {
		t.Fatalf("download+verify failed: %v", err)
	}
	defer os.Remove(newBinary)

	if err := r.Replace(currentExe, newBinary, "v1.0.0", "v2.0.0"); err != nil {
		t.Fatalf("replace failed: %v", err)
	}

	replaced, err := os.ReadFile(currentExe)
	if err != nil {
		t.Fatalf("read replaced exe: %v", err)
	}
	if string(replaced) != string(binaryContent) {
		t.Fatalf("replaced content mismatch: got %q", string(replaced))
	}

	backup := filepath.Join(upgradesDir, "cs-cloud.bak")
	backupContent, err := os.ReadFile(backup)
	if err != nil {
		t.Fatalf("read backup: %v", err)
	}
	if string(backupContent) != string(oldContent) {
		t.Fatalf("backup content mismatch")
	}

	state, err := r.LoadState()
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if state.Status != "pending_verify" {
		t.Errorf("status=%q, want pending_verify", state.Status)
	}
	if state.PreviousVersion != "v1.0.0" {
		t.Errorf("previous=%q", state.PreviousVersion)
	}
	if state.CurrentVersion != "v2.0.0" {
		t.Errorf("current=%q", state.CurrentVersion)
	}
}

func TestDownloadSHA256Mismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("bad content"))
	}))
	defer srv.Close()

	dir := t.TempDir()
	d := NewDownloader(dir)
	_, err := d.Download(context.Background(), srv.URL, "wrongsha256")
	if err == nil {
		t.Fatal("expected sha256 mismatch error")
	}
	if !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDownloadHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	dir := t.TempDir()
	d := NewDownloader(dir)
	_, err := d.Download(context.Background(), srv.URL, "")
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !strings.Contains(err.Error(), "status 500") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDownloadCancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.Write([]byte("data"))
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dir := t.TempDir()
	d := NewDownloader(dir)
	_, err := d.Download(ctx, srv.URL, "")
	if err == nil {
		t.Fatal("expected context cancelled error")
	}
}

func TestCheckerBadURL(t *testing.T) {
	c := NewChecker("://invalid-url")
	_, err := c.Check(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestCheckerNon200Response(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("not found"))
	}))
	defer srv.Close()

	c := NewChecker(srv.URL)
	_, err := c.Check(context.Background())
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "status 404") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReplacerBackupAndReplace(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	dir := t.TempDir()
	currentExe := filepath.Join(dir, "cs-cloud")
	os.WriteFile(currentExe, []byte("old-binary"), 0o755)

	newBinary := filepath.Join(dir, "cs-cloud-new")
	os.WriteFile(newBinary, []byte("new-binary"), 0o755)

	r := NewReplacer(filepath.Join(dir, "upgrades"))
	err := r.Replace(currentExe, newBinary, "v1.0.0", "v2.0.0")
	if err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(currentExe)
	if string(content) != "new-binary" {
		t.Errorf("exe content=%q, want new-binary", string(content))
	}

	backup := filepath.Join(dir, "upgrades", "cs-cloud.bak")
	bakContent, _ := os.ReadFile(backup)
	if string(bakContent) != "old-binary" {
		t.Errorf("backup content=%q, want old-binary", string(bakContent))
	}
}

func TestReplacerRollbackFromBackup(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	dir := t.TempDir()
	currentExe := filepath.Join(dir, "cs-cloud")
	os.WriteFile(currentExe, []byte("new-binary"), 0o755)

	r := NewReplacer(filepath.Join(dir, "upgrades"))
	os.MkdirAll(filepath.Join(dir, "upgrades"), 0o755)
	os.WriteFile(filepath.Join(dir, "upgrades", "cs-cloud.bak"), []byte("old-binary"), 0o755)

	if err := r.Rollback(currentExe); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(currentExe)
	if string(content) != "old-binary" {
		t.Errorf("exe content=%q, want old-binary", string(content))
	}

	state, _ := r.LoadState()
	if state == nil || state.Status != "rolled_back" {
		t.Errorf("expected rolled_back status")
	}
}

func TestReplacerRollbackNoBackup(t *testing.T) {
	dir := t.TempDir()
	r := NewReplacer(filepath.Join(dir, "upgrades"))
	err := r.Rollback(filepath.Join(dir, "nonexistent"))
	if err == nil {
		t.Fatal("expected error when no backup")
	}
}

func TestReplacerMarkVerified(t *testing.T) {
	dir := t.TempDir()
	r := NewReplacer(dir)
	r.saveState(&UpgradeState{
		PreviousVersion: "v1.0.0",
		CurrentVersion:  "v2.0.0",
		Status:          "pending_verify",
	})

	if err := r.MarkVerified(); err != nil {
		t.Fatal(err)
	}

	state, _ := r.LoadState()
	if state.Status != "completed" {
		t.Errorf("status=%q, want completed", state.Status)
	}
}

func TestReplacerCleanup(t *testing.T) {
	dir := t.TempDir()
	r := NewReplacer(dir)

	os.WriteFile(filepath.Join(dir, "cs-cloud.bak"), []byte("backup"), 0o644)

	r.Cleanup()

	if _, err := os.Stat(filepath.Join(dir, "cs-cloud.bak")); err == nil {
		t.Error("backup should be cleaned up")
	}
}

func TestManagerConcurrentApplyProtection(t *testing.T) {
	dir := t.TempDir()
	mgr := &Manager{
		checker:    NewChecker("http://localhost:1"),
		downloader: NewDownloader(filepath.Join(dir, "tmp")),
		verifier:   NewVerifier(),
		replacer:   NewReplacer(filepath.Join(dir, "upgrades")),
		policy:     PolicyAuto,
		interval:   6 * time.Hour,
		running:    true,
		RestartCh:  make(chan struct{}, 1),
	}

	err := mgr.Apply(context.Background(), "")
	if err == nil {
		t.Fatal("expected concurrent protection error")
	}
	if !strings.Contains(err.Error(), "already in progress") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestManagerApplyNoUpdate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"can_update": false,
			"version":   "v1.0.0",
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	mgr := NewManager(srv.URL, dir)

	err := mgr.Apply(context.Background(), "")
	if err == nil {
		t.Fatal("expected no update error")
	}
	if !strings.Contains(err.Error(), "no update available") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestManagerApplyVersionMismatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"can_update":    true,
			"version":      "v2.0.0",
			"download_url": "http://localhost:1/file",
			"sha256":       "abc",
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	mgr := NewManager(srv.URL, dir)

	err := mgr.Apply(context.Background(), "v3.0.0")
	if err == nil {
		t.Fatal("expected version mismatch error")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCopyFile(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst")
	content := []byte("test file content for copy")

	os.WriteFile(src, content, 0o644)
	if err := copyFile(src, dst); err != nil {
		t.Fatal(err)
	}

	result, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(result) != string(content) {
		t.Errorf("copy mismatch: got %q", string(result))
	}
}

func TestCopyFileNotExist(t *testing.T) {
	err := copyFile("/nonexistent/file", "/nonexistent/dst")
	if err == nil {
		t.Fatal("expected error for nonexistent src")
	}
}

func TestHistoryAppend(t *testing.T) {
	dir := t.TempDir()
	r := NewReplacer(dir)

	entry1 := &UpgradeState{
		PreviousVersion: "v1.0.0",
		CurrentVersion:  "v1.1.0",
		UpgradedAt:      "2026-04-11T00:00:00Z",
		Status:          "completed",
	}
	entry2 := &UpgradeState{
		PreviousVersion: "v1.1.0",
		CurrentVersion:  "v1.2.0",
		UpgradedAt:      "2026-04-12T00:00:00Z",
		Status:          "completed",
	}

	if err := r.AppendHistory(entry1); err != nil {
		t.Fatal(err)
	}
	if err := r.AppendHistory(entry2); err != nil {
		t.Fatal(err)
	}

	history, err := r.LoadHistory()
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("expected 2 history entries, got %d", len(history))
	}
	if history[0].CurrentVersion != "v1.1.0" {
		t.Errorf("first entry version=%q", history[0].CurrentVersion)
	}
	if history[1].CurrentVersion != "v1.2.0" {
		t.Errorf("second entry version=%q", history[1].CurrentVersion)
	}
}

func TestDownloadCleanupOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	dir := t.TempDir()
	d := NewDownloader(dir)
	_, err := d.Download(context.Background(), srv.URL, "")
	if err == nil {
		t.Fatal("expected error")
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Error("temp file should be cleaned up on error")
		}
	}
}

func TestVerifierEmptyFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "empty")
	os.WriteFile(f, []byte{}, 0o644)

	v := NewVerifier()
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte{}))
	if err := v.VerifySHA256(f, hash); err != nil {
		t.Fatal(err)
	}
}

func TestDownloadSucceedsWithoutSHA256(t *testing.T) {
	content := []byte("binary data")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(content)
	}))
	defer srv.Close()

	dir := t.TempDir()
	d := NewDownloader(dir)
	path, err := d.Download(context.Background(), srv.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(path)

	result, _ := os.ReadFile(path)
	if string(result) != string(content) {
		t.Errorf("downloaded content mismatch: got %q", string(result))
	}
}

func TestManagerDownloadAndVerifyNoURL(t *testing.T) {
	dir := t.TempDir()
	mgr := NewManager("http://localhost:1", dir)
	result := &CheckResult{CanUpdate: true, Version: "v2.0.0"}

	_, err := mgr.downloadAndVerify(context.Background(), result)
	if err == nil {
		t.Fatal("expected error for empty download_url")
	}
	if !strings.Contains(err.Error(), "no download url") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConcurrentReplacerAccess(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping on windows")
	}

	dir := t.TempDir()
	r := NewReplacer(filepath.Join(dir, "upgrades"))

	var wg sync.WaitGroup
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := r.LoadState()
			if err != nil {
				errors <- err
			}
		}(i)
	}
	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent access error: %v", err)
	}
}

func TestCheckerWithCancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		json.NewEncoder(w).Encode(map[string]any{"can_update": false})
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := NewChecker(srv.URL)
	_, err := c.Check(ctx)
	if err == nil {
		t.Fatal("expected context cancelled error")
	}
}

func TestCheckResultUserAgent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua := r.Header.Get("User-Agent")
		if ua == "" {
			t.Error("User-Agent header should be set")
		}
		if !strings.HasPrefix(ua, "cs-cloud/") {
			t.Errorf("User-Agent=%q, expected cs-cloud/ prefix", ua)
		}
		io.ReadAll(r.Body)
		json.NewEncoder(w).Encode(map[string]any{
			"can_update": true,
			"version":   "v2.0.0",
		})
	}))
	defer srv.Close()

	c := NewChecker(srv.URL)
	c.Check(context.Background())
}

func TestDownloadVerifySuccessFlow(t *testing.T) {
	content := []byte("verified binary content")
	expectedSHA := fmt.Sprintf("%x", sha256.Sum256(content))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(content)
	}))
	defer srv.Close()

	dir := t.TempDir()
	d := NewDownloader(dir)
	path, err := d.Download(context.Background(), srv.URL, expectedSHA)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	defer os.Remove(path)

	v := NewVerifier()
	if err := v.VerifySHA256(path, expectedSHA); err != nil {
		t.Fatalf("verify failed: %v", err)
	}
}

func TestIOCopyBuffer(t *testing.T) {
	src, _ := os.CreateTemp(t.TempDir(), "test-*.src")
	defer src.Close()
	dst, _ := os.CreateTemp(t.TempDir(), "test-*.dst")
	defer dst.Close()

	data := make([]byte, 128*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	src.Write(data)
	src.Seek(0, 0)

	_, err := io.Copy(dst, src)
	if err != nil {
		t.Fatalf("io.Copy error: %v", err)
	}

	dst.Seek(0, 0)
	result, _ := io.ReadAll(dst)
	if len(result) != len(data) {
		t.Errorf("length mismatch: got %d, want %d", len(result), len(data))
	}
}
