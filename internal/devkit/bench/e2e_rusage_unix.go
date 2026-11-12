//go:build !windows

package bench

import (
	"os/exec"
	"syscall"
)

// maxRSSKiB reads the finished child's peak RSS; CI runs Linux only, so the macOS byte-vs-KiB difference in ru_maxrss is moot here.
func maxRSSKiB(cmd *exec.Cmd) int64 {
	if cmd.ProcessState == nil {
		return 0
	}
	ru, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	if !ok {
		return 0
	}
	return int64(ru.Maxrss)
}
