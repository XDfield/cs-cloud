//go:build !windows

package terminal

import (
	"io"
	"os"
	"os/exec"
	"syscall"

	"cs-cloud/internal/logger"
	"github.com/creack/pty"
)

func startPty(shell string, cwd string, rows, cols uint16) (io.ReadWriteCloser, int, error) {
	ptmx, pid, err := startPtyInner(shell, cwd, rows, cols, true)
	if err != nil && IsPermissionError(err) {
		logger.Warn("terminal: PTY with Setpgid failed (%v), retrying without", err)
		return startPtyInner(shell, cwd, rows, cols, false)
	}
	return ptmx, pid, err
}

func startPtyInner(shell string, cwd string, rows, cols uint16, setpgid bool) (io.ReadWriteCloser, int, error) {
	cmd := exec.Command(shell)
	if cwd != "" {
		cmd.Dir = cwd
	}
	cmd.Env = os.Environ()
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: setpgid}

	ptmx, err := pty.StartWithSize(cmd, &pty.Winsize{
		Rows: rows,
		Cols: cols,
	})
	if err != nil {
		return nil, 0, err
	}

	return ptmx, cmd.Process.Pid, nil
}

func (s *Session) resizePty(rows, cols uint16) error {
	f, ok := s.ptmx.(*os.File)
	if !ok {
		return nil
	}
	return pty.Setsize(f, &pty.Winsize{Rows: rows, Cols: cols})
}
