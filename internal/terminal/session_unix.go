//go:build !windows

package terminal

import (
	"os"
	"os/exec"
	"io"

	"github.com/creack/pty"
)

func startPty(shell string, cwd string, rows, cols uint16) (io.ReadWriteCloser, int, error) {
	cmd := exec.Command(shell)
	if cwd != "" {
		cmd.Dir = cwd
	}
	cmd.Env = os.Environ()

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
