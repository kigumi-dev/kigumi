//go:build !linux

package hostshell

import "syscall"

func glibcText(_ syscall.Errno, text string) string { return text }
