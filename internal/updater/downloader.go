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

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
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

	contentLength := resp.ContentLength
	hasher := sha256.New()
	w := io.MultiWriter(tmpFile, hasher)

	if contentLength > 0 {
		p := newDownloadProgress(contentLength)
		if _, err := io.Copy(w, &progressWriter{w: resp.Body, p: p, total: contentLength}); err != nil {
		p.quit()
		return "", fmt.Errorf("write download: %w", err)
	}
		p.finish()
	} else {
		if _, err := io.Copy(w, resp.Body); err != nil {
			return "", fmt.Errorf("write download: %w", err)
		}
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

type progressWriter struct {
	w     io.Reader
	p     *downloadProgress
	total int64
	wrote int64
}

func (pw *progressWriter) Read(b []byte) (int, error) {
	n, err := pw.w.Read(b)
	pw.wrote += int64(n)
	pw.p.setProgress(float64(pw.wrote) / float64(pw.total))
	return n, err
}

type downloadProgress struct {
	prog    progress.Model
	pct     float64
	finished bool
	program *tea.Program
}

func newDownloadProgress(total int64) *downloadProgress {
	prog := progress.New(progress.WithGradient("#7D56F4", "#04B575"), progress.WithWidth(40))
	dp := &downloadProgress{prog: prog}
	dp.program = tea.NewProgram(dp)
	go dp.program.Run()
	return dp
}

func (dp *downloadProgress) setProgress(pct float64) {
	dp.pct = pct
	dp.program.Send(progressMsg(pct))
}

func (dp *downloadProgress) finish() {
	dp.finished = true
	dp.program.Send(progressMsg(1.0))
	time.Sleep(300 * time.Millisecond)
	dp.program.Quit()
}

func (dp *downloadProgress) quit() {
	dp.program.Quit()
}

type progressMsg float64

func (dp downloadProgress) Init() tea.Cmd {
	return nil
}

func (dp downloadProgress) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case progressMsg:
		dp.pct = float64(msg)
		if dp.pct >= 1.0 {
			return dp, tea.Quit
		}
		return dp, nil
	case tea.WindowSizeMsg:
		dp.prog.Width = msg.Width - 20
		if dp.prog.Width < 10 {
			dp.prog.Width = 10
		}
		return dp, nil
	}
	return dp, nil
}

func (dp downloadProgress) View() string {
	bar := dp.prog.ViewAs(dp.pct)
	pct := fmt.Sprintf("%5.1f%%", dp.pct*100)
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#B0B0B0"))
	if dp.finished {
		return labelStyle.Render("  Downloading") + " " + bar + " " + lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Render("done")
	}
	return labelStyle.Render("  Downloading") + " " + bar + " " + pct
}
