//go:build linux || darwin || freebsd || netbsd || openbsd

package repl

import (
	"os"
	"syscall"
)

// replSignals are the signals the interactive loop watches; SIGWINCH wakes a
// blocked read so the prompt can be redrawn after a resize.
func replSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGWINCH}
}
