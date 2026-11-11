//go:build unix

package vm_test

import "syscall"

// cpuTime is the process's user plus system CPU in nanoseconds, which a loaded machine does not stretch the way wall time is.
func cpuTime() int64 {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return ru.Utime.Nano() + ru.Stime.Nano()
}
