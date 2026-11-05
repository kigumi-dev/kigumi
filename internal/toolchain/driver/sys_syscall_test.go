package driver_test

import (
	"os/exec"
	"runtime"
	"slices"
	"testing"
)

// TestSysSyscallWrappers exercises the std/sys libc syscall bridge on the one
// ABI whose getpid number this test can name: x86-64 Linux.
func TestSysSyscallWrappers(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("getpid's syscall number is asserted for x86-64 Linux only")
	}
	src := `import sys from std/sys

fn main() {
    let pid = sys.call0(39).unwrap()
    print "pid=${pid > 0}"
    print "bad=${sys.call0(999999) is Err(_)}"
}
`
	exe := buildProgram(t, src)
	out, err := exec.Command(exe).Output()
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	got := string(out)
	want := []byte("pid=true\nbad=true\n")
	if !slices.Equal([]byte(got), want) {
		t.Fatalf("got %q, want %q", got, want)
	}
}
