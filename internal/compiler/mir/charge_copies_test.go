package mir_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
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

fn keepN(x: Named) -> Named {
    x
}

fn handKept(x: Named) -> Int {
    keepN(x).n
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

fn main() -> Void! {}
`

func TestPlanDirectChargesAPlannedCopy(t *testing.T) {
	p := planned(t, chargeSrc)
	for name, want := range map[string]bool{"main.written": false, "main.readOnly": true, "main.hand": true, "main.handKept": false} {
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
			if in.Op != midir.OpAlias || f.Locals[in.Dst].Name != "b" {
				continue
			}
			f.Blocks[bi].Insts[k].Op, f.Blocks[bi].Insts[k].Copy = midir.OpShare, true
			ve, ok := p.Verify().(*mir.Error)
			if !ok || ve.Rule != "alloc-claim" || !strings.Contains(ve.Msg, "copies Named") {
				t.Fatalf("got %v, want an alloc-claim error naming the copy", ve)
			}
			return
		}
	}
	t.Fatalf("no binding of b in readOnly:\n%s", p.Dump())
}
