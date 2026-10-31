//go:build darwin || freebsd

package driver

import (
	"os"
	"syscall"
)

// changeTime is the inode change time, which cp -p, touch -r and rsync -t cannot restore along with the mtime; 0 when unknown.
func changeTime(st os.FileInfo) int64 {
	if s, ok := st.Sys().(*syscall.Stat_t); ok {
		return s.Ctimespec.Nano()
	}
	return 0
}
