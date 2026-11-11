//go:build linux

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

// target32Program is the AOT smoke test for a 32-bit pointer target: it
// covers the 64-bit division the C runtime implements itself there, 128-bit
// arithmetic, floats, arrays and the C pointer width the target reports.
const target32Program = `import ffi from std/ffi

fn main() {
    let nullPtr = ffi.null[u8]()
    print "ptr=${ffi.sizeOf(&nullPtr)}"
    let a: i64 = -9223372036854775807 - 1
    let b: i64 = 3
    print "div=${a / b} rem=${a % b}"
    let u: u64 = 18446744073709551615
    print "udiv=${u / 7} urem=${u % 7}"
    let x: i64 = 123456789
    let y: i64 = 1000
    print "mul=${x * y}"
    let w: i128 = 170141183460469231731687303715884105727
    print "wide=${w / 3} ${w % 10}"
    let f = 1.25 * 3.0
    print "float=${f}"
    let values = Array.of(1, 2, 3)
    let n = values.len()
    let mut total = 0
    for v in values {
        total = total + v
    }
    print "sum=${total} len=${n}"
}
`

const target32Want = `ptr=4
div=-3074457345618258602 rem=-2
udiv=2635249153387078802 urem=1
mul=123456789000
wide=56713727820156410577229101238628035242 7
float=3.75
sum=6 len=3
`

// TestTarget32CrossBuildRuns builds a hosted executable for each 32-bit
// pointer target this host can execute and compares its output with the
// 64-bit expectation. linux/386 runs natively on an amd64 kernel; linux/arm
// needs qemu-arm, and both fall back to the user-mode emulator when the host
// cannot execute the binary itself.
func TestTarget32CrossBuildRuns(t *testing.T) {
	requireZig(t)
	for _, c := range []struct{ triple, arch, qemu string }{
		{triple: "x86-linux", arch: "386", qemu: "qemu-i386"},
		{triple: "arm-linux", arch: "arm", qemu: "qemu-arm"},
	} {
		t.Run(c.triple, func(t *testing.T) {
			t.Parallel()
			runner := target32Runner(t, c.arch, c.qemu)
			root := t.TempDir()
			os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"run32\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
			os.WriteFile(filepath.Join(root, "main.kg"), []byte(target32Program), 0o644)
			target, err := buildconfig.ParseTarget(testCompiler(), c.triple, "")
			if err != nil {
				t.Fatal(err)
			}
			std, _ := filepath.Abs("../../../std")
			m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
			if err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(root, "run32")
			var buildErr bytes.Buffer
			if ok, err := driver.BuildWith(m, out, &buildErr, driver.BuildOptions{Target: target}); err != nil || !ok {
				t.Fatalf("build for %s: %v\n%s", c.triple, err, buildErr.String())
			}
			argv := append(runner, out)
			cmd := exec.Command(argv[0], argv[1:]...)
			cmd.Dir = root
			got, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("run %s: %v\n%s", strings.Join(argv, " "), err, got)
			}
			if string(got) != target32Want {
				t.Fatalf("%s output mismatch\ngot:\n%s\nwant:\n%s", c.triple, got, target32Want)
			}
		})
	}
}

// target32Runner is the command prefix that executes an arch binary: nothing
// when the host kernel can run it, else the matching qemu user emulator. A
// host with neither skips the target rather than pretending to have run it.
func target32Runner(t *testing.T, arch, qemu string) []string {
	t.Helper()
	if (arch == "386" && runtime.GOARCH == "amd64") || (arch == "arm" && runtime.GOARCH == "arm") {
		return nil
	}
	path, err := exec.LookPath(qemu)
	if err != nil {
		t.Skipf("%s is not on PATH and this host cannot run %s binaries", qemu, arch)
	}
	return []string{path}
}
