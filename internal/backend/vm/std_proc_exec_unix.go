//go:build unix

package vm

import (
	"errors"
	"syscall"
)

// execFailedBeforeRun tells a child's execvp failure apart from a fork
// failure in a Start error. A native build forks and then runs execvp in the
// child, so an exec failure never reaches the parent: the child exits 127 and
// only a failed fork is a spawn error. Go starts the child itself and returns
// the errno from either step, so spawn treats every errno but resource
// exhaustion, which is what a failing fork reports, as the child's 127. An
// error without an errno is the PATH lookup failing, which execvp would also
// fail in the child.
func execFailedBeforeRun(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return true
	}
	switch errno {
	case syscall.EAGAIN, syscall.ENOMEM, syscall.EMFILE, syscall.ENFILE:
		return false
	}
	return true
}
