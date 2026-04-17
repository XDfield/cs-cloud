//go:build windows

package terminal

func IsPermissionError(err error) bool {
	return false
}
