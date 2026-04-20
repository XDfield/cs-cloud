//go:build !windows

package cli

import "os"

func openNullDevice() (*os.File, error) {
	return os.Open("/dev/null")
}
