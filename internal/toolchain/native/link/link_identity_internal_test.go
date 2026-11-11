package link

import (
	"slices"
	"testing"
)

// TestDefaultLibraryDirsFollowHostArch checks the Debian multiarch search
// path comes from the host architecture rather than the hardcoded x86 pair
// it used to carry.
func TestDefaultLibraryDirsFollowHostArch(t *testing.T) {
	amd := defaultLibraryDirs("linux", "amd64")
	for _, want := range []string{"/usr/lib", "/usr/lib/x86_64-linux-gnu", "/usr/lib64", "/lib", "/lib/x86_64-linux-gnu", "/usr/local/lib"} {
		if !slices.Contains(amd, want) {
			t.Errorf("linux/amd64 search path lacks %q: %v", want, amd)
		}
	}
	if slices.Contains(amd, "/usr/lib/aarch64-linux-gnu") {
		t.Errorf("linux/amd64 search path still carries the foreign arch directory: %v", amd)
	}
	arm := defaultLibraryDirs("linux", "arm64")
	if !slices.Contains(arm, "/usr/lib/aarch64-linux-gnu") || !slices.Contains(arm, "/lib/aarch64-linux-gnu") {
		t.Errorf("linux/arm64 search path = %v", arm)
	}
	s390 := defaultLibraryDirs("linux", "s390x")
	if !slices.Contains(s390, "/usr/lib/s390x-linux-gnu") {
		t.Errorf("linux/s390x search path = %v", s390)
	}
	if got := defaultLibraryDirs("windows", "amd64"); got != nil {
		t.Errorf("windows has no default library dirs, got %v", got)
	}
	if got := defaultLibraryDirs("darwin", "arm64"); !slices.Contains(got, "/opt/homebrew/lib") {
		t.Errorf("darwin search path = %v", got)
	}
	unknown := defaultLibraryDirs("freebsd", "sparc64")
	if !slices.Contains(unknown, "/usr/lib") || slices.Contains(unknown, "/usr/lib/") {
		t.Errorf("an architecture without a multiarch directory should search only the plain paths: %v", unknown)
	}
}
