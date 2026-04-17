package updater

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"cs-cloud/internal/logger"
	"cs-cloud/internal/version"
)

type Policy int

const (
	PolicyAuto     Policy = iota
	PolicyDownload
	PolicyManual
)

type Manager struct {
	checker      *Checker
	downloader   *Downloader
	verifier     *Verifier
	replacer     *Replacer
	policy       Policy
	interval     time.Duration
	upgradesDir  string
	mu           sync.Mutex
	running      bool
	lastCheck    time.Time
	lastResult   *CheckResult
	RestartCh    chan struct{}
}

type Option func(*Manager)

func WithPolicy(p Policy) Option {
	return func(m *Manager) { m.policy = p }
}

func WithInterval(d time.Duration) Option {
	return func(m *Manager) { m.interval = d }
}

func NewManager(cloudBaseURL, rootDir string, opts ...Option) *Manager {
	upgradesDir := filepath.Join(rootDir, "upgrades")
	exe, _ := os.Executable()
	exeDir := ""
	if exe != "" {
		exeDir = filepath.Dir(exe)
	}
	m := &Manager{
		checker:     NewChecker(cloudBaseURL),
		downloader:  NewDownloader(filepath.Join(exeDir, ".cs-cloud-update")),
		verifier:    NewVerifier(),
		replacer:    NewReplacer(upgradesDir),
		policy:      PolicyAuto,
		interval:    6 * time.Hour,
		upgradesDir: upgradesDir,
		RestartCh:   make(chan struct{}, 1),
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

func (m *Manager) Run(ctx context.Context) {
	m.verifyOnStartup()

	ticker := time.NewTicker(m.interval)
	defer ticker.Stop()

	m.doCheck(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.doCheck(ctx)
		}
	}
}

func (m *Manager) CheckNow(ctx context.Context) (*CheckResult, error) {
	return m.checker.Check(ctx)
}

func (m *Manager) Apply(ctx context.Context, targetVersion string) error {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return fmt.Errorf("upgrade already in progress")
	}
	m.running = true
	m.mu.Unlock()

	defer func() {
		m.mu.Lock()
		m.running = false
		m.mu.Unlock()
	}()

	result, err := m.checker.Check(ctx)
	if err != nil {
		return fmt.Errorf("check: %w", err)
	}
	if !result.CanUpdate {
		return fmt.Errorf("no update available")
	}
	if targetVersion != "" && result.Version != targetVersion {
		return fmt.Errorf("available version %s does not match requested %s", result.Version, targetVersion)
	}

	return m.executeUpgrade(ctx, result)
}

func (m *Manager) Rollback() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve exe: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return fmt.Errorf("resolve exe symlink: %w", err)
	}
	if err := m.replacer.Rollback(exe); err != nil {
		return err
	}
	logger.Info("rollback completed")
	return nil
}

func (m *Manager) History() (*UpgradeState, error) {
	return m.replacer.LoadState()
}

func (m *Manager) FullHistory() ([]*UpgradeState, error) {
	return m.replacer.LoadHistory()
}

func (m *Manager) LastCheck() (time.Time, *CheckResult) {
	return m.lastCheck, m.lastResult
}

func (m *Manager) doCheck(ctx context.Context) {
	result, err := m.checker.Check(ctx)
	if err != nil {
		logger.Error("update check failed: %v", err)
		return
	}
	m.lastCheck = time.Now()
	m.lastResult = result

	if !result.CanUpdate {
		logger.Info("no update available (current: %s)", version.Get())
		return
	}

	logger.Info("update available: %s → %s (force: %v)", version.Get(), result.Version, result.Force)

	if m.policy == PolicyManual {
		return
	}

	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.mu.Unlock()

	go func() {
		defer func() {
			m.mu.Lock()
			m.running = false
			m.mu.Unlock()
		}()
		if m.policy == PolicyDownload {
			logger.Info("update downloaded, waiting for manual apply (version: %s)", result.Version)
			_, err := m.downloadAndVerify(ctx, result)
			if err != nil {
				logger.Error("download failed: %v", err)
			}
			return
		}
		if err := m.executeUpgrade(ctx, result); err != nil {
			logger.Error("auto upgrade failed: %v", err)
		}
	}()
}

func (m *Manager) executeUpgrade(ctx context.Context, result *CheckResult) error {
	logger.Info("starting upgrade to %s", result.Version)

	newBinary, err := m.downloadAndVerify(ctx, result)
	if err != nil {
		return err
	}

	exe, err := os.Executable()
	if err != nil {
		cleanupFile(newBinary)
		return fmt.Errorf("resolve exe: %w", err)
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		cleanupFile(newBinary)
		return fmt.Errorf("resolve exe symlink: %w", err)
	}

	if err := m.replacer.Replace(exe, newBinary, version.Get(), result.Version); err != nil {
		cleanupFile(newBinary)
		return fmt.Errorf("replace: %w", err)
	}

	logger.Info("upgrade to %s completed, requesting restart", result.Version)
	select {
	case m.RestartCh <- struct{}{}:
	default:
	}
	return nil
}

func (m *Manager) downloadAndVerify(ctx context.Context, result *CheckResult) (string, error) {
	if result.DownloadURL == "" {
		return "", fmt.Errorf("no download url in check result")
	}

	logger.Info("downloading %s ...", result.Version)
	path, err := m.downloader.Download(ctx, result.DownloadURL, result.SHA256)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}

	logger.Info("download verified (sha256 ok)")
	return path, nil
}

func (m *Manager) verifyOnStartup() {
	state, err := m.replacer.LoadState()
	if err != nil || state == nil {
		return
	}
	if state.Status != "pending_verify" {
		return
	}

	logger.Info("verifying pending upgrade %s → %s", state.PreviousVersion, state.CurrentVersion)

	if state.CurrentVersion != version.Get() {
		logger.Error("version mismatch after upgrade, expected %s but running %s, rolling back", state.CurrentVersion, version.Get())
		exe, _ := os.Executable()
		if exe != "" {
			exe, _ = filepath.EvalSymlinks(exe)
			if rerr := m.replacer.Rollback(exe); rerr != nil {
				logger.Error("rollback failed: %v", rerr)
			}
		}
		return
	}

	if err := m.replacer.MarkVerified(); err != nil {
		logger.Error("mark verified failed: %v", err)
		return
	}

	m.replacer.AppendHistory(&UpgradeState{
		PreviousVersion: state.PreviousVersion,
		CurrentVersion:  state.CurrentVersion,
		UpgradedAt:      state.UpgradedAt,
		Status:          "completed",
	})

	m.replacer.Cleanup()
	logger.Info("upgrade to %s verified successfully", state.CurrentVersion)
}

func cleanupFile(path string) {
	if path != "" {
		os.Remove(path)
	}
}

func PlatformString() string {
	return fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
}
