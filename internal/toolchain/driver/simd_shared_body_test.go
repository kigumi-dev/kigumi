package driver_test

import (
	"bytes"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// A shared SIMD body the drop glue reaches is a diagnostic on both backends, never an llgen internal error.
func TestSimdSharedBodyIsADiagnosticOnBothBackends(t *testing.T) {
	root := t.TempDir()
	writeEntryScopeModule(t, root, map[string]string{"main.kg": `import simd from std/simd
type W[const N: usize] = resource { pub v simd.Vec[i32, N] }
fn W[N].drop(move self) { print "drop ${self.v.add(self.v).lane(0)}" }
let w = W[4] { v: simd.Vec.splat[i32, 4](3) }
print "made ${w.v.lane(1)}"
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
	if err != nil || code == 0 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "E1713") || !strings.Contains(stderr.String(), "`W.drop`") {
		t.Fatalf("VM: err=%v exit=%d stdout=%q stderr=%q", err, code, stdout.String(), stderr.String())
	}
	stderr.Reset()
	if _, ok, err := driver.Compile(load(), &stderr); err != nil || ok || !strings.Contains(stderr.String(), "E1713") {
		t.Fatalf("native: ok=%v err=%v\n%s", ok, err, stderr.String())
	}
}
