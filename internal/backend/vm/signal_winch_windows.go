//go:build windows

package vm

import "syscall"

// windowChangeSignal is 0 on Windows: console resize is a console event, not a
// signal, so a WindowChange watch is registered without a delivery source.
func windowChangeSignal() syscall.Signal { return 0 }
