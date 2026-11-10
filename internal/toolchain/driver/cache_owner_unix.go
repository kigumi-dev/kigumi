//go:build unix

package driver

import (
	"os"
	"syscall"
)

// ownedBySelf says dir belongs to this account; an owner-only mode means nothing if someone else owns the directory it applies to.
func ownedBySelf(dir string) bool {
	fi, err := os.Lstat(dir)
	if err != nil {
		return false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return true
	}
	return int(st.Uid) == os.Getuid()
}
