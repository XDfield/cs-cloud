//go:build !windows

package terminal

import (
	"errors"
	"strings"
	"syscall"
)

func IsPermissionError(err error) bool {
	if errors.Is(err, syscall.EPERM) {
		return true
	}
	return strings.Contains(err.Error(), "operation not permitted")
}
