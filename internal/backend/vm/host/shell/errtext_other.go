//go:build !linux

package shell

import "syscall"

func glibcText(_ syscall.Errno, text string) string { return text }
