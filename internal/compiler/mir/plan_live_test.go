package mir_test

import (
	"fmt"
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

const liveJoinSrc = `
type Tick = Up(n: Int) | Down(n: Int) | Flat

type Big = {
    pub a Int
    pub b Int
}

type Holder = Full(big: Big, t: Tick) | Empty

fn shape(h: Option[Holder]) -> String {
    if h is Some(Full(Big { a: 2, b: _ }, Up(2))) {
        "two"
    } else if h is Some(Full(_, Down(_))) {
        "down"
    } else {
        "other"
    }
}

fn pick(xs: &Array[fn(Int) -> Int], n: Int) -> Int {
    (xs.get(0).unwrap())(n)
}

fn firstLarge(xs: &Array[Int]) -> Int {
    let mut found = 0
    for k in 0..xs.len() {
        if xs.get(k) is Some(v) {
            if v > 10 {
                found = v
                break
            }
        }
    }
    found
}

fn main() -> Void! {}
`

// ownedTemps are the temporaries a path must give up exactly once: fresh values nothing else adopts.
func ownedTemps(p *mir.Program, f *midir.Func) map[midir.LocalID]bool {
	out := map[midir.LocalID]bool{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if f.Locals[in.Dst].Ent != 0 || !in.Op.Yields() {
				continue
			}
			if in.Op == midir.OpPayload {
				out[in.Dst] = true
			}
		}
	}
	return out
}

type walk struct {
	p     *mir.Program
	f     *midir.Func
	temps map[midir.LocalID]bool
	seen  map[string]bool
	errs  []string
}

func (w *walk) diverges(b int) bool {
	for _, in := range w.f.Blocks[b].Insts {
		if in.Op == midir.OpPanic || in.Op.Yields() && w.f.Locals[in.Dst].Type == sem.TyNever {
			return true
		}
	}
	return false
}

func (w *walk) run(b int, owned map[midir.LocalID]bool) {
	key := fmt.Sprint(b, owned)
	if w.seen[key] {
		return
	}
	w.seen[key] = true
	owned = cloneSet(owned)
	blk := w.f.Blocks[b]
	for k, in := range blk.Insts {
		for i, a := range in.Args {
			if !w.temps[a] {
				continue
			}
			if !owned[a] {
				w.errs = append(w.errs, fmt.Sprintf("b%d/%d reads %%%s while it is not owned", b, k, w.f.Locals[a].Name))
			}
			if in.Op == midir.OpRelease || w.p.ArgConsumed(w.f, in, i) {
				delete(owned, a)
			}
		}
		if w.temps[in.Dst] && in.Op.Yields() && in.Op != midir.OpRelease {
			if owned[in.Dst] {
				w.errs = append(w.errs, fmt.Sprintf("b%d/%d redefines %%%s while it is owned", b, k, w.f.Locals[in.Dst].Name))
			}
			owned[in.Dst] = true
		}
	}
	if w.diverges(b) {
		return
	}
	for _, a := range blk.Term.Args {
		delete(owned, a)
	}
	if blk.Term.Op == midir.TermReturn {
		for t := range owned {
			w.errs = append(w.errs, fmt.Sprintf("returns from b%d still owning %%%s", b, w.f.Locals[t].Name))
		}
	}
	for _, t := range blk.Term.Targets {
		w.run(int(t), owned)
	}
}

func cloneSet(s map[midir.LocalID]bool) map[midir.LocalID]bool {
	out := make(map[midir.LocalID]bool, len(s))
	for k, v := range s {
		out[k] = v
	}
	return out
}

func TestPlanReleasesEveryPathExactlyOnce(t *testing.T) {
	p := planned(t, liveJoinSrc)
	for _, name := range []string{"main.shape", "main.pick", "main.firstLarge"} {
		f := funcNamed(t, p, name)
		temps := ownedTemps(p, f)
		if len(temps) == 0 && name != "main.pick" {
			t.Fatalf("%s: no payload temporaries to follow", name)
		}
		w := &walk{p: p, f: f, temps: temps, seen: map[string]bool{}}
		w.run(0, map[midir.LocalID]bool{})
		for _, e := range w.errs {
			t.Errorf("%s: %s\n%s", name, e, p.Dump())
		}
	}
}
