package mir_test

import (
	"strings"
	"testing"

	"kigumi/internal/mir"
)

const chargeSrc = `
type Named = {
    pub name String
    pub mut n Int
}

type Holder = {
    pub mut q Named
    pub k Int
}

fn readN(x: Named) -> Int {
    x.n
}

fn hand(x: Named) -> Int {
    readN(x)
}

fn written(h: &Holder) -> Int {
    let mut b = h.q
    b.n = 2
    b.n + h.q.n
}

fn readOnly(h: &Holder) -> Int {
    let b = h.q
    b.n + h.k
}

fn main() -> Unit! {}
`

func TestPlanDirectChargesAPlannedCopy(t *testing.T) {
	p := planned(t, chargeSrc)
	for name, want := range map[string]bool{"main.written": false, "main.readOnly": true, "main.hand": false} {
		if got := p.R.Summary(funcNamed(t, p, name).Ent).Noalloc; got != want {
			t.Errorf("%s noalloc = %v, want %v:\n%s", name, got, want, p.Dump())
		}
	}
}

func TestVerifyAllocClaimRejectsAnUnchargedCopy(t *testing.T) {
	p := planned(t, chargeSrc)
	f := funcNamed(t, p, "main.readOnly")
	if !p.R.Summary(f.Ent).Noalloc {
		t.Fatalf("readOnly should claim noalloc")
	}
	for bi := range f.Blocks {
		for k, in := range f.Blocks[bi].Insts {
			if in.Op != mir.OpAlias || f.Locals[in.Dst].Name != "b" {
				continue
			}
			f.Blocks[bi].Insts[k].Op, f.Blocks[bi].Insts[k].Copy = mir.OpShare, true
			ve, ok := p.Verify().(*mir.Error)
			if !ok || ve.Rule != "alloc-claim" || !strings.Contains(ve.Msg, "copies Named") {
				t.Fatalf("got %v, want an alloc-claim error naming the copy", ve)
			}
			return
		}
	}
	t.Fatalf("no binding of b in readOnly:\n%s", p.Dump())
}
