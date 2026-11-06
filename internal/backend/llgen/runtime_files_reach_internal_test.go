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
