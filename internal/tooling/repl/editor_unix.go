//go:build linux || darwin || freebsd || netbsd || openbsd

package repl

import (
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// termState is the terminal state to restore on Close.
type termState struct {
	fd   int
	mode unix.Termios
}

// IOCTL request names differ between Linux and the BSDs; the per-OS files
// editor_ioctl_linux.go and editor_ioctl_bsd.go pick the right pair.

func enableRaw(in *os.File, _ io.Writer) (*termState, error) {
	fd := int(in.Fd())
	term, err := unix.IoctlGetTermios(fd, ioctlReadTermios)
	if err != nil {
		return nil, err
	}
	raw := *term
	raw.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	raw.Oflag &^= unix.OPOST
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Cflag &^= unix.CSIZE | unix.PARENB
	raw.Cflag |= unix.CS8
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0
	if err := unix.IoctlSetTermios(fd, ioctlWriteTermios, &raw); err != nil {
		return nil, err
	}
	return &termState{fd: fd, mode: *term}, nil
}

func restoreRaw(state *termState) error {
	return unix.IoctlSetTermios(state.fd, ioctlWriteTermios, &state.mode)
}
