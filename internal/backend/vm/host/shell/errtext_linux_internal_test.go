//go:build linux

package shell

import (
	"syscall"
	"testing"
)

func TestOSErrTextReadsLikeGlibc(t *testing.T) {
	for errno, want := range map[syscall.Errno]string{
		syscall.ENOENT:       "No such file or directory",
		syscall.EIO:          "Input/output error",
		syscall.ENAMETOOLONG: "File name too long",
		syscall.ESTALE:       "Stale file handle",
		syscall.Errno(41):    "Unknown error 41",
		syscall.Errno(140):   "Unknown error 140",
	} {
		if got := OSErrText(errno); got != want {
			t.Errorf("OSErrText(%d) = %q, want %q", errno, got, want)
		}
	}
}
