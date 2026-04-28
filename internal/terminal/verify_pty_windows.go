//go:build windows

package terminal

func verifyShellPty(path string) bool {
	return true
}
