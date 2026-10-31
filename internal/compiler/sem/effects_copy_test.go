package sem_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
)

func TestChargeCopiesReportsWhatItBreaks(t *testing.T) {
	res := checkIsr(t, `
fn quiet(n: Int) -> Int {
    n
}

noalloc fn claims(n: Int) -> Int {
    quiet(n)
}

fn other(n: Int) -> Int {
    n
}

export(interrupt) {
    pub fn tick() {
        let _ = quiet(1)
    }
}
`)
	if res.HasErrors() {
		t.Fatalf("unexpected errors before the charge:\n%s", res.Render())
	}
	main := res.PackageByPath("main")
	quiet, other := res.PackageMember(main, "quiet"), res.PackageMember(main, "other")
	e := res.Entity(quiet)
	site := sem.CopySite{Owner: quiet, File: e.File, Node: e.Node, Type: sem.TyBytes}
	res.ChargeCopies([]sem.CopySite{site})
	res.ChargeCopies([]sem.CopySite{site})
	out := res.Render()
	for _, want := range []string{"error[E907]: `quiet` is not `noalloc`", "error[E1080]: export(interrupt) handler `tick` may allocate"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	if n := strings.Count(out, "error["); n != 2 {
		t.Errorf("got %d errors, want 2:\n%s", n, out)
	}
	if res.Summary(quiet).Noalloc || !res.Summary(other).Noalloc {
		t.Errorf("only quiet should lose noalloc")
	}
	ex := res.ExplainOpFact(quiet, sem.FactAlloc)
	if !ex.Maybe || !strings.Contains(ex.Chain[len(ex.Chain)-1].Text, "copies `Bytes` into a box of its own") {
		t.Errorf("explain = %+v", ex)
	}
}
