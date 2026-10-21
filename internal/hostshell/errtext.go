package hostshell

import (
	"errors"
	"os/exec"
	"strings"
	"syscall"
)

// OSErrText is an OS error as the native runtime shows it: libc's strerror text for the errno, without the operation and path Go adds.
func OSErrText(err error) string {
	if errors.Is(err, exec.ErrNotFound) {
		err = syscall.ENOENT
	}
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return err.Error()
	}
	text := errno.Error()
	return strings.ToUpper(text[:1]) + text[1:]
}
