//go:build windows

package repl

import (
	"os"
	"syscall"
)

// replSignals are the signals the interactive loop watches. Windows has no
// SIGWINCH; console resize is not delivered as a signal, so there is no
// reason to register it.
func replSignals() []os.Signal {
	return []os.Signal{os.Interrupt, syscall.SIGTERM, syscall.SIGHUP}
}
