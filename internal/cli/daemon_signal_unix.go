//go:build !windows

package cli

import (
	"os/signal"
	"syscall"
)

func configureDaemonSignals() {
	signal.Ignore(syscall.SIGHUP)
}
