package driver_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/toolchain/driver"
)

const claimSource = `type Point = {
    pub x Int
    pub y Int
}

interface Taker {
    noalloc fn take(self, p: Point) -> Int
}

type Pm = {
    pub k Int
}

noalloc fn Pm.take(self, p: Point) -> Int { p.x + self.k }

fn Pm.make(self, i: Int) -> Point { Point { x: i, y: self.k } }

type Named = {
    pub name String
}

noalloc fn Named.take(self, p: Point) -> Int { p.y }

fn throughValue(i: Int) -> Int {
    let pm = Pm { k: 1 }
    let f = pm.make
    f(i).y
}

fn throughTable(m: &Taker, i: Int) -> Int { m.take(Point { x: i, y: 2 }) }

fn throughWitness[T: Taker](m: &T, i: Int) -> Int { m.take(Point { x: i, y: 2 }) }

fn keep(ps: Array[Point], p: Point) -> Array[Point] {
    let mut out = ps
    out.push(p)
    out
}

fn main() -> Void! {
    let t: Taker = Pm { k: 3 }
    let nm = Named { name: "a" }
    print "${throughValue(1)} ${throughTable(&t, 2)} ${throughWitness(&nm, 3)} ${keep(Array.empty[Point](), Point { x: 1, y: 1 }).len()}"
}
`

func claimCodes(claims []mirverify.AllocClaim) []string {
	var out []string
	for _, c := range claims {
		out = append(out, c.Diag.Code+" "+c.Diag.Msg)
	}
	return out
}

// Each body boxes through a method value, a method table or a shared generic body; the summary now says so.
func TestBoxingThroughValuesAndTablesIsNoLongerNoalloc(t *testing.T) {
	t.Parallel()
	a, err := driver.Analyze(loadTryConstructorModule(t, claimSource))
	if err != nil {
		t.Fatal(err)
	}
	if a.Res.HasErrors() {
		t.Fatalf("a body the summary charges is reported:\n%s", a.Res.Render())
	}
	for _, name := range []string{"throughValue", "throughTable", "throughWitness"} {
		if a.Res.Summary(mainFn(t, a.Res, name)).Noalloc {
			t.Errorf("%s boxes, yet its summary says noalloc", name)
		}
	}
}

// A summary that claims noalloc for a body that boxes reaches E1770 through the built program.
func TestAForgedNoallocSummaryReachesE1770FromSource(t *testing.T) {
	t.Parallel()
	a, err := driver.Analyze(loadTryConstructorModule(t, claimSource))
	if err != nil || a.Res.HasErrors() {
		t.Fatalf("%v\n%s", err, a.Res.Render())
	}
	for _, name := range []string{"keep", "throughValue"} {
		info := a.Res.Fn(mainFn(t, a.Res, name))
		info.Summary.Noalloc, info.Summary.Known = true, true
	}
	got := strings.Join(claimCodes(mirverify.CheckAllocClaims(a.Prog, mir.DemoteBoxedLends(a.Prog))), "\n")
	for _, want := range []string{"E1770 `keep` is inferred `noalloc`", "E1770 `throughValue` is inferred `noalloc`", "E1843 `throughValue` is inferred `noalloc`"} {
		if !strings.Contains(got, want) {
			t.Errorf("no %q among the claims:\n%s", want, got)
		}
	}
}
