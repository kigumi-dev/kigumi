package vm

import (
	"os"
	"testing"
)

// The Go runtime replaces a closed 0/1/2 with /dev/null before main, so the closed-at-start probe has to tell that
// placeholder (O_RDWR) from the /dev/null a shell redirects (O_RDONLY).
func TestFDClosedAtStart(t *testing.T) {
	rw, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer rw.Close()
	if !fdClosedAtStart(rw.Fd()) {
		t.Errorf("/dev/null opened O_RDWR is the runtime's closed-descriptor placeholder, want closed")
	}

	ro, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	if fdClosedAtStart(ro.Fd()) {
		t.Errorf("/dev/null opened O_RDONLY is a redirection, not a closed descriptor")
	}

	file, err := os.CreateTemp(t.TempDir(), "fd")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if fdClosedAtStart(file.Fd()) {
		t.Errorf("a regular file is not a closed descriptor")
	}
}
