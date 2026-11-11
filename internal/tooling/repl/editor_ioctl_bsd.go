//go:build darwin || freebsd || netbsd || openbsd

package repl

import "golang.org/x/sys/unix"

// The BSDs, including darwin, spell the termios ioctls TIOCGETA/TIOCSETA
// instead of Linux's TCGETS/TCSETS.
const (
	ioctlReadTermios  = unix.TIOCGETA
	ioctlWriteTermios = unix.TIOCSETA
)
