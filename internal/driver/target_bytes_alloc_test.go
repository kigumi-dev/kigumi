package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

const wideBytesSrc = `let big: usize = 4294967296
let n = big + 16
match Bytes.tryZeros(n) {
    Ok(z) -> print "zeros ${z.len()}"
    Err(_) -> print "zeros err"
}
match Bytes.tryFill(7, n) {
    Ok(z) -> print "fill ${z.len()}"
    Err(_) -> print "fill err"
}
`

const nearLimitBytesSrc = `let n: usize = 4294967290
match Bytes.tryZeros(n) {
    Ok(z) -> print "zeros ${z.len()}"
    Err(_) -> print "zeros err"
}
match Bytes.tryFill(7, n) {
    Ok(z) -> print "fill ${z.len()}"
    Err(_) -> print "fill err"
}
`

const wideBytesFromSrc = `import ffi from std/ffi

let src = Bytes.zeros(4)
let n: usize = 4294967300
print "${unsafe { ffi.bytesFrom(ffi.bytesPtr(&src), n) }.len()}"
`

// A usize at or above 2^32 must make Bytes allocation fail on a 32-bit
// target instead of wrapping to a short buffer with the long length.
func TestBytesAllocationRefusesAboveSizeT(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	cases := []struct{ name, src, want string }{
		{"try", wideBytesSrc, "zeros err\nfill err\n"},
		{"nearLimit", nearLimitBytesSrc, "zeros err\nfill err\n"},
		{"bytesFrom", wideBytesFromSrc, "panic: out of memory\n"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, runErr := runX86(t, c.src)
			if !strings.HasPrefix(got, strings.TrimSuffix(c.want, "\n")) {
				t.Errorf("got %q, want prefix %q (run: %v)", got, c.want, runErr)
			}
		})
	}
}

func runX86(t *testing.T, src string) (string, error) {
	t.Helper()
	target, err := driver.ParseTarget(testCompiler(), "x86-linux-musl", "")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"widebytes\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(root, "prog")
	var buildErr bytes.Buffer
	if ok, err := driver.BuildWith(m, exe, &buildErr, driver.BuildOptions{Target: target}); err != nil || !ok {
		t.Fatalf("build: %v\n%s", err, buildErr.String())
	}
	var out bytes.Buffer
	cmd := exec.Command(exe)
	cmd.Stdout = &out
	cmd.Stderr = &out
	runErr := cmd.Run()
	if runErr != nil && strings.Contains(runErr.Error(), "exec format") {
		t.Skip("host cannot run 32-bit x86 binaries")
	}
	return out.String(), runErr
}
