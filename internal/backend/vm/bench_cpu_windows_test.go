//go:build windows

package vm_test

import "golang.org/x/sys/windows"

// cpuTime is the process's user plus kernel time in nanoseconds, read through
// GetProcessTimes; it is the Windows twin of getrusage(RUSAGE_SELF).
func cpuTime() int64 {
	process := windows.CurrentProcess()
	var creation, exit, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(process, &creation, &exit, &kernel, &user); err != nil {
		return 0
	}
	return kernel.Nanoseconds() + user.Nanoseconds()
}
