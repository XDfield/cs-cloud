package updater

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"cs-cloud/internal/version"
)

type Downloader struct {
	httpClient *http.Client
	tmpDir     string
}

func NewDownloader(tmpDir string) *Downloader {
	return &Downloader{
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
		tmpDir: tmpDir,
	}
}

func (d *Downloader) Download(ctx context.Context, url, expectedSHA256 string) (string, error) {
	if err := os.MkdirAll(d.tmpDir, 0o755); err != nil {
		return "", fmt.Errorf("create tmp dir: %w", err)
	}

	tmpFile, err := os.CreateTemp(d.tmpDir, "cs-cloud-update-*.tmp")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() {
		tmpFile.Close()
		if _, err := os.Stat(tmpPath); err == nil {
			os.Remove(tmpPath)
		}
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create download request: %w", err)
	}
	req.Header.Set("User-Agent", version.UserAgent())

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download binary: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed (status %d)", resp.StatusCode)
	}

	hasher := sha256.New()
	w := io.MultiWriter(tmpFile, hasher)

	if _, err := io.Copy(w, resp.Body); err != nil {
		return "", fmt.Errorf("write download: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		return "", fmt.Errorf("sync download: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("close download: %w", err)
	}

	actual := fmt.Sprintf("%x", hasher.Sum(nil))
	if expectedSHA256 != "" && actual != expectedSHA256 {
		os.Remove(tmpPath)
		return "", fmt.Errorf("sha256 mismatch: expected %s, got %s", expectedSHA256, actual)
	}

	dst := filepath.Join(d.tmpDir, "cs-cloud-new")
	if err := os.Rename(tmpPath, dst); err != nil {
		return "", fmt.Errorf("rename temp file: %w", err)
	}

	if err := os.Chmod(dst, 0o755); err != nil {
		os.Remove(dst)
		return "", fmt.Errorf("chmod binary: %w", err)
	}

	return dst, nil
}
