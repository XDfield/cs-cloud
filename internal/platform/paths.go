package platform

import (
	"os"
	"path/filepath"
	"sync"
)

var (
	mu       sync.RWMutex
	authPath string
	dataDir  string
)

func SetAuthPath(p string) {
	mu.Lock()
	defer mu.Unlock()
	if p != "" && !filepath.IsAbs(p) {
		abs, err := filepath.Abs(p)
		if err == nil {
			p = abs
		}
	}
	authPath = p
}

func AuthPath() string {
	mu.RLock()
	defer mu.RUnlock()
	return authPath
}

func SetDataDir(p string) {
	mu.Lock()
	defer mu.Unlock()
	if p != "" && !filepath.IsAbs(p) {
		abs, err := filepath.Abs(p)
		if err == nil {
			p = abs
		}
	}
	dataDir = p
}

func DataDir() string {
	mu.RLock()
	defer mu.RUnlock()
	return dataDir
}

func AppDir() string {
	mu.RLock()
	d := dataDir
	mu.RUnlock()
	if d != "" {
		return filepath.Join(d, "cs-cloud")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".costrict", "cs-cloud")
}

func CoStrictShareDir() string {
	if p := AuthPath(); p != "" {
		return filepath.Dir(p)
	}
	mu.RLock()
	d := dataDir
	mu.RUnlock()
	if d != "" {
		return filepath.Join(d, "share")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".costrict", "share")
}
