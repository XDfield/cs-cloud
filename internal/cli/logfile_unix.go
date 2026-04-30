//go:build !windows

package cli

import "os"

func openFileShared(path string) (*os.File, error) {
	return os.Open(path)
}
