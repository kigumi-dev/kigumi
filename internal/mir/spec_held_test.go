package mir_test

import (
	"fmt"
	"strings"
	"testing"

	"kigumi/internal/diag"
)

// phantomFan instantiates generics whose type argument changes nothing, so no instance gains over its shared body.
func phantomFan(levels int, root string) string {
	var src strings.Builder
	for i := 0; i < levels-1; i++ {
		fmt.Fprintf(&src, "fn g%d[T](n: Int) -> Int { g%d[(T, Int)](n).wrappingAdd(g%d[(Int, T)](n)) }\n", i, i+1, i+1)
	}
	fmt.Fprintf(&src, "fn g%d[T](n: Int) -> Int { n }\n\n%s\n\nfn main() -> Void! {}\n", levels-1, root)
	return src.String()
}

func TestSpecializeKeepsWhatAHeldRootReaches(t *testing.T) {
	for _, tc := range []struct {
		name, root string
		want       int
	}{
		{"noalloc", "noalloc fn run(n: Int) -> Int { g0[Int](n) }", 32},
		{"interrupt", "export(interrupt) {\n    pub fn tick() {\n        let n = g0[Int](1)\n    }\n}", 32},
		{"through a plain fn", "fn plain(n: Int) -> Int { g0[Int](n) }\n\nnoalloc fn run(n: Int) -> Int { plain(n) }", 32},
		{"plain", "fn run(n: Int) -> Int { g0[Int](n) }", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := specialized(t, phantomFan(6, tc.root))
			if len(p.Diags) != 0 {
				t.Fatalf("diagnostics = %+v, want none", p.Diags)
			}
			if n := len(instances(p)); n != tc.want {
				t.Errorf("%d instantiations kept, want %d:\n%s", n, tc.want, strings.Join(instances(p), "\n"))
			}
		})
	}
}

func TestSpecializeHoldsANoallocRootPastTheBudget(t *testing.T) {
	var src strings.Builder
	for i := range 300 {
		fmt.Fprintf(&src, "noalloc fn leaf%d[T: Copy](a: &Array[T]) -> T? { a.last() }\n", i)
	}
	src.WriteString("noalloc fn hub[T: Copy](a: &Array[T]) -> Int {\n    let mut c = 0\n")
	for i := range 300 {
		fmt.Fprintf(&src, "    if leaf%d(a) is Some(_) { c = c + 1 }\n", i)
	}
	src.WriteString("    c\n}\n\nnoalloc fn useHub(a: &Array[Int]) -> Int { hub(a) }\n\nfn main() -> Void! {}\n")
	p := specialized(t, src.String())
	if len(p.Diags) != 0 {
		t.Fatalf("diagnostics = %+v, want none", p.Diags)
	}
	if n := len(instances(p)); n != 301 {
		t.Errorf("%d instantiations kept, want 301", n)
	}
}

func TestSpecializeReportsTheHeldLimit(t *testing.T) {
	p := specialized(t, phantomFan(16, "noalloc fn run(n: Int) -> Int { g0[Int](n) }"))
	if len(p.Diags) != 1 || p.Diags[0].Diag.Code != "E2269" || p.Diags[0].Diag.Severity != diag.Error {
		t.Fatalf("diagnostics = %+v, want one E2269 error", p.Diags)
	}
	if msg := p.Diags[0].Diag.Msg; !strings.Contains(msg, "the `noalloc` fn `run` needs every instantiation") || !strings.Contains(msg, "past 4096 of them") {
		t.Errorf("message %q", msg)
	}
	if err := p.Verify(); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestSpecializeCountsAnInstanceAHeldRootAdopts(t *testing.T) {
	root := "fn plain(n: Int) -> Int { g0[Int](n) }\n\nnoalloc fn run(n: Int) -> Int { g0[Int](n) }"
	p := specialized(t, phantomFan(14, root))
	if len(p.Diags) != 1 || p.Diags[0].Diag.Code != "E2269" {
		t.Fatalf("diagnostics = %+v, want one E2269", p.Diags)
	}
	if n := len(instances(p)); n != 4096 {
		t.Errorf("%d instantiations kept, want 4096 with the adopted one counted", n)
	}
}
