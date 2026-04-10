package platform

import (
	"os"
	"path/filepath"
	"sync"
)

var (
	mu       sync.RWMutex
	authPath string
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

func DataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".cs-cloud")
}

func CoStrictShareDir() string {
	if p := AuthPath(); p != "" {
		return filepath.Dir(p)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".costrict", "share")
}
