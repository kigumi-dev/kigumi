package driver_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

var invalidDerefForms = []struct{ name, src string }{
	{"let", "fn f(p: &Int) -> Int {\n    let q = *p\n    q\n}\n\nfn main() -> Void! {}\n"},
	{"interpolation", "fn f(p: &Int) -> String {\n    \"${*p}\"\n}\n\nfn main() -> Void! {}\n"},
	{"argument", "fn g(x: Int) -> Int { x }\n\nfn f(p: &Int) -> Int {\n    g(*p)\n}\n\nfn main() -> Void! {}\n"},
	{"top level", "let x = 3\nlet p = &x\nlet q = *p\n"},
}

// A tree that did not parse is reported by the checker's result, so no pass after it sees a missing node.
func TestInvalidDereferenceStopsBeforeLowering(t *testing.T) {
	t.Parallel()
	for _, c := range invalidDerefForms {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			a, err := driver.Analyze(loadTryConstructorModule(t, c.src))
			if err != nil {
				t.Fatalf("analysis fault: %v", err)
			}
			if !a.Res.HasErrors() || a.Prog != nil {
				t.Fatalf("errors=%v, program lowered=%v", a.Res.HasErrors(), a.Prog != nil)
			}
			if !strings.Contains(a.Res.Render(), "expected expression, found `*`") {
				t.Errorf("the parse error is not rendered:\n%s", a.Res.Render())
			}
			var out, errOut bytes.Buffer
			code, err := driver.Run(loadTryConstructorModule(t, c.src), &out, &errOut, nil)
			if err != nil || code != 1 || !strings.Contains(errOut.String(), "expected expression, found `*`") {
				t.Errorf("run: code=%d err=%v stderr=%q", code, err, errOut.String())
			}
			var buildErr bytes.Buffer
			ok, err := driver.Build(loadTryConstructorModule(t, c.src), filepath.Join(t.TempDir(), "prog"), &buildErr)
			if err != nil || ok || !strings.Contains(buildErr.String(), "expected expression, found `*`") {
				t.Errorf("build: ok=%v err=%v stderr=%q", ok, err, buildErr.String())
			}
		})
	}
}
