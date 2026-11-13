package llgen

import (
	"testing"

	mirreach "kigumi/internal/compiler/mir/reach"
)

// TestRuntimeFilesSignalGatingMatchesReach keeps the runtime's POSIX signal OS
// set in step with the capability layer's copy of it.
func TestRuntimeFilesSignalGatingMatchesReach(t *testing.T) {
	for _, os := range []string{"linux", "darwin", "freebsd", "netbsd", "openbsd", "wasi", "windows", "freestanding", "uefi", "android", "plan9"} {
		has := false
		for _, f := range RuntimeFiles("posix", os) {
			if f.Name == "rt_signal_hosted.c" {
				has = true
			}
		}
		if want := mirreach.PosixSignalOS(os); has != want {
			t.Errorf("RuntimeFiles(\"posix\", %q) includes rt_signal_hosted.c = %v, mirreach.PosixSignalOS = %v", os, has, want)
		}
	}
}

// TestRuntimeFilesTargetGating keeps the target-only translation units out of
// builds that cannot use them: no FS TU on a bare target, and no guard TU on a
// POSIX OS where it would compile only empty stubs.
func TestRuntimeFilesTargetGating(t *testing.T) {
	for _, tc := range []struct {
		sys, os string
		fs      bool
		guard   bool
	}{
		{"posix", "linux", true, true},
		{"posix", "darwin", true, true},
		{"posix", "freebsd", true, true},
		{"posix", "netbsd", true, false},
		{"posix", "openbsd", true, false},
		{"posix", "windows", true, false},
		{"posix", "wasi", true, false},
		{"none", "linux", false, false},
		{"none", "freestanding", false, false},
		{"posix", "uefi", false, false},
	} {
		hasFS, hasGuard := false, false
		for _, f := range RuntimeFiles(tc.sys, tc.os) {
			hasFS = hasFS || f.Name == "rt_fs.c"
			hasGuard = hasGuard || f.Name == "rt_stack_guard_hosted.c"
		}
		if hasFS != tc.fs || hasGuard != tc.guard {
			t.Errorf("RuntimeFiles(%q, %q): fs=%v guard=%v, want fs=%v guard=%v", tc.sys, tc.os, hasFS, hasGuard, tc.fs, tc.guard)
		}
	}
}
