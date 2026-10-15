package vm

import (
	"syscall"
	"testing"
)

func TestReleaseSocketClosesOnce(t *testing.T) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Skip(err)
	}
	rec := mkRecord(0, []*obj{mkInt(int64(fd), 0)})
	releaseSocket(rec)
	reused, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer syscall.Close(reused)
	if reused != fd {
		t.Skipf("fd %d not reused (got %d)", fd, reused)
	}
	releaseSocket(rec)
	var st syscall.Stat_t
	if err := syscall.Fstat(reused, &st); err != nil {
		t.Fatalf("second release closed an fd opened after the first: %v", err)
	}
}
