//go:build linux || darwin || freebsd || netbsd || openbsd

package vm

import "golang.org/x/sys/unix"

// pollReadable is poll(2) on one descriptor, retried across EINTR.
func pollReadable(fd int, timeoutMs int64) (bool, error) {
	if timeoutMs < 0 {
		timeoutMs = 0
	}
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	for {
		n, err := unix.Poll(fds, int(timeoutMs))
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return false, err
		}
		return n > 0, nil
	}
}
