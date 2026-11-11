//go:build linux || darwin || freebsd || netbsd || openbsd

package vm

import "syscall"

// windowChangeSignal is the signal a terminal sends when its size changes.
func windowChangeSignal() syscall.Signal { return syscall.SIGWINCH }
