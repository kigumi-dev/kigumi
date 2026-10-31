//go:build windows

package bench

import "os/exec"

// maxRSSKiB has no portable rusage on this platform; CI runs Linux only.
func maxRSSKiB(cmd *exec.Cmd) int64 { return 0 }
