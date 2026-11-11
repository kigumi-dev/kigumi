//go:build windows

package vm

import "golang.org/x/sys/windows"

// pollReadable waits on the stdin handle until it has input or the timeout
// expires. Console input handles and pipes are waitable, and a regular file is
// always signaled, which matches poll(POLLIN) for the file case readSome
// cares about.
func pollReadable(fd int, timeoutMs int64) (bool, error) {
	if timeoutMs < 0 {
		timeoutMs = 0
	}
	if timeoutMs > int64(^uint32(0)) {
		timeoutMs = int64(^uint32(0))
	}
	event, err := windows.WaitForSingleObject(windows.Handle(uintptr(fd)), uint32(timeoutMs))
	switch event {
	case windows.WAIT_OBJECT_0:
		return true, nil
	case uint32(windows.WAIT_TIMEOUT):
		return false, nil
	}
	return false, err
}
