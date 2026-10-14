package mir_test

import (
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

const liveCallsSrc = `
fn a(n: Int) -> String { "a${n}" }

fn b(n: Int) -> Result[String, String] {
    if n % 3 == 0 { fail "b${n}" } else { Ok("b${n}") }
}

fn join3(x: String, y: String, z: String) -> String { "${x}${y}${z}" }

fn merged(k: Int) -> String {
    "${if k == 0 { a(0) } else if k == 1 { a(1) } else { a(2) }}"
}

fn run(n: Int) -> Result[String, String] {
    let r = join3(a(n), b(n)?, a(n + 1))
    Ok(r)
}

fn main() -> Unit! {}
`

// freshStrings are the String temporaries a call or a temp-to-temp alias hands over.
func freshStrings(f *mir.Func) map[mir.LocalID]bool {
	out := map[mir.LocalID]bool{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if f.Locals[in.Dst].Ent != 0 || f.Locals[in.Dst].Type != sem.TyString {
				continue
			}
			if in.Op == mir.OpCall || in.Op == mir.OpAlias && f.Locals[in.Args[0]].Ent == 0 {
				out[in.Dst] = true
			}
		}
	}
	return out
}

func TestPlanReleasesCallResultsOnEveryPath(t *testing.T) {
	p := planned(t, liveCallsSrc)
	for _, name := range []string{"main.merged", "main.run"} {
		f := funcNamed(t, p, name)
		temps := freshStrings(f)
		if len(temps) == 0 {
			t.Fatalf("%s: no String temporaries to follow", name)
		}
		w := &walk{p: p, f: f, temps: temps, seen: map[string]bool{}}
		w.run(0, map[mir.LocalID]bool{})
		for _, e := range w.errs {
			t.Errorf("%s: %s\n%s", name, e, p.Dump())
		}
	}
}
