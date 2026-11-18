package driver_test

import (
	"bytes"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// A SIMD body that runs shared, here behind a type test that opens its lane type, is a diagnostic on both backends,
// never an llgen internal error.
func TestSimdSharedBodyIsADiagnosticOnBothBackends(t *testing.T) {
	root := t.TempDir()
	writeEntryScopeModule(t, root, map[string]string{"main.kg": `import simd from std/simd
interface Sh { fn run(self) -> Int }
type H[T: Copy] = { pub v simd.Vec[T, 4] }
fn H[T: Copy].run(self) -> Int { _ = self.v.add(self.v); 1 }
fn probe(s: &Sh) -> Int { if s is H(x) { x.run() } else { 0 } }
let s: Sh = H[i32] { v: simd.Vec.splat[i32, 4](1) }
print "${probe(&s)}"
`})
	load := func() *driver.Module {
		m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std"})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	var stdout, stderr bytes.Buffer
	code, err := driver.RunWith(load(), driver.RunOptions{Cwd: root}, &stdout, &stderr, nil)
	if err != nil || code == 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "E1713") || !strings.Contains(stderr.String(), "`H.run`") {
		t.Fatalf("VM: err=%v exit=%d stdout=%q stderr=%q", err, code, stdout.String(), stderr.String())
	}
	stderr.Reset()
	if _, ok, err := driver.Compile(load(), &stderr); err != nil || ok || !strings.Contains(stderr.String(), "E1713") {
		t.Fatalf("native: ok=%v err=%v\n%s", ok, err, stderr.String())
	}
}
