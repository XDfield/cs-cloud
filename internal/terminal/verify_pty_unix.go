//go:build !windows

package terminal

func verifyShellPty(path string) bool {
	ptmx, pid, err := startPty(path, "", 1, 1)
	if err != nil {
		return false
	}
	ptmx.Close()
	killProcessTree(pid)
	return true
}
