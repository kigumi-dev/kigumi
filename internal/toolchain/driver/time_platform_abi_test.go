package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

// timeABISrc prints the clock selectors std/time passes on and the C layout
// of the timespec the runtime fills through the target's <time.h>.
const timeABISrc = `import ffi from std/ffi
import platform from std/platform

let ts = platform.Timespec { sec: 0, nsec: 0, pad: 0 }
print "${platform.clockRealtime()} ${platform.clockMonotonic()} ${ffi.sizeOf(&ts)} ${ffi.alignOf(&ts)}"
`

// TestTimePlatformABI pins the timespec layout per target against the C
// library each one links: it is 16 bytes with a 64-bit tv_sec on every target
// std/time supports (musl and WASI are time64, the BSDs and Darwin LP64 or
// time64, Windows LLP64), never two pointer-sized fields. x86-linux-musl
// rounds the record to 4-byte alignment, wasm32 and the Arm ABI to 8.
func TestTimePlatformABI(t *testing.T) {
	t.Parallel()
	cases := []struct {
		triple string
		want   string
	}{
		{"x86_64-linux-musl", "0 1 16 8\n"},
		{"x86-linux-musl", "0 1 16 4\n"},
		{"arm-linux-musleabihf", "0 1 16 8\n"},
		{"wasm32-wasi", "0 1 16 8\n"},
		{"x86_64-freebsd", "0 1 16 8\n"},
		{"aarch64-macos", "0 1 16 8\n"},
		{"x86_64-netbsd", "0 1 16 8\n"},
		{"x86_64-openbsd", "0 1 16 8\n"},
		{"x86_64-windows-gnu", "0 1 16 8\n"},
	}
	std, _ := filepath.Abs("../../../std")
	for _, c := range cases {
		t.Run(c.triple, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"timeabi\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
			os.WriteFile(filepath.Join(root, "main.kg"), []byte(timeABISrc), 0o644)
			target, err := buildconfig.ParseTarget(testCompiler(), c.triple, "")
			if err != nil {
				t.Fatal(err)
			}
			m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
			if err != nil {
				t.Fatal(err)
			}
			var out, errOut bytes.Buffer
			code, err := driver.Run(m, &out, &errOut, nil)
			if err != nil {
				t.Fatalf("run: %v\n%s", err, errOut.String())
			}
			if code != 0 || out.String() != c.want {
				t.Fatalf("got exit %d output %q (want %q)\n%s", code, out.String(), c.want, errOut.String())
			}
		})
	}
}

// TestTimeReadsClockThroughPlatform keeps the libc call in std/platform: the
// clock ids, the time64 symbol redirection and wasi-libc's pointer clockid_t
// belong to the target's <time.h>, not to std/time.
func TestTimeReadsClockThroughPlatform(t *testing.T) {
	t.Parallel()
	src, err := os.ReadFile("../../../std/time/time_hosted.kg")
	if err != nil {
		t.Fatal(err)
	}
	text := string(src)
	for _, want := range []string{"platform.clockRealtime()", "platform.clockMonotonic()", "platform.clockGettime(", "platform.nanosleep("} {
		if !strings.Contains(text, want) {
			t.Errorf("std/time/time_hosted.kg does not use %s", want)
		}
	}
	for _, unwanted := range []string{"clock_gettime(", "fn nanosleep("} {
		if strings.Contains(text, unwanted) {
			t.Errorf("std/time/time_hosted.kg calls %s directly; the platform seam owns it", unwanted)
		}
	}
}

// TestTime32CrossBuildRuns drives the time64 ABI on a real 32-bit binary: a
// monotonic reading, a 120 ms sleep and the wall clock must all work where
// the old isize pair made monotonic() trap and sleep() return at once.
func TestTime32CrossBuildRuns(t *testing.T) {
	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		t.Skip("needs an amd64 Linux kernel to run an i386 binary")
	}
	requireZig(t)
	root := t.TempDir()
	src := `import time from std/time

fn main() {
    let a = time.monotonic()
    let wall = time.now().unixMillis()
    time.sleep(120)
    let d = time.monotonic().diffMillis(a)
    print "mono=${d >= 50} ${d < 10000}"
    print "wall=${time.now().unixMillis() > wall}"
}
`
	if err := os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"time32\"\n    kigumi: \"0.1\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	std, _ := filepath.Abs("../../../std")
	target, err := buildconfig.ParseTarget(testCompiler(), "x86-linux-musl", "")
	if err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "prog32")
	var buildErr bytes.Buffer
	if ok, err := testBuild(m, exe, &buildErr); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	out, err := exec.Command(exe).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s", err, out)
	}
	if want := "mono=true true\nwall=true\n"; string(out) != want {
		t.Fatalf("got %q, want %q", out, want)
	}
}
