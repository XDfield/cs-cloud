//go:build windows

package terminal

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/UserExistsError/conpty"
)

type winPty struct {
	cpty *conpty.ConPty
}

func (w *winPty) Read(p []byte) (int, error) {
	return w.cpty.Read(p)
}

func (w *winPty) Write(p []byte) (int, error) {
	return w.cpty.Write(p)
}

func (w *winPty) Close() error {
	return w.cpty.Close()
}

func startPty(shell string, cwd string, rows, cols uint16) (io.ReadWriteCloser, int, error) {
	if !conpty.IsConPtyAvailable() {
		return nil, 0, fmt.Errorf("terminal: ConPTY not available on this Windows version (requires Windows 10 1809+)")
	}

	commandLine := buildCommandLine(shell)

	opts := []conpty.ConPtyOption{
		conpty.ConPtyDimensions(int(cols), int(rows)),
	}
	if cwd != "" {
		opts = append(opts, conpty.ConPtyWorkDir(cwd))
	}
	env := os.Environ()
	env = appendUTF8Env(env)
	opts = append(opts, conpty.ConPtyEnv(env))

	cpty, err := conpty.Start(commandLine, opts...)
	if err != nil {
		return nil, 0, fmt.Errorf("terminal: conpty start: %w", err)
	}

	w := &winPty{cpty: cpty}
	return w, cpty.Pid(), nil
}

func (s *Session) resizePty(rows, cols uint16) error {
	w, ok := s.ptmx.(*winPty)
	if !ok {
		return nil
	}
	return w.cpty.Resize(int(cols), int(rows))
}

func buildCommandLine(shell string) string {
	if strings.Contains(shell, " ") {
		shell = fmt.Sprintf(`"%s"`, shell)
	}

	lower := strings.ToLower(shell)
	if strings.Contains(lower, "cmd.exe") || strings.Contains(lower, "cmd") {
		return shell + " /k chcp 65001 >nul"
	}
	return shell
}

func appendUTF8Env(env []string) []string {
	localeKeys := map[string]string{
		"LC_ALL":   "C.UTF-8",
		"LC_CTYPE": "C.UTF-8",
		"LANG":     "C.UTF-8",
	}
	set := make(map[string]bool)
	for _, e := range env {
		for k := range localeKeys {
			if strings.HasPrefix(e, k+"=") {
				set[k] = true
			}
		}
	}
	for k, v := range localeKeys {
		if !set[k] {
			env = append(env, k+"="+v)
		}
	}
	return env
}
