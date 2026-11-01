package plan_test

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/mirfixture"
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

fn main() -> Void! {}
`

// freshStrings are the String temporaries a call or a temp-to-temp alias hands over.
func freshStrings(f *midir.Func) map[midir.LocalID]bool {
	out := map[midir.LocalID]bool{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if f.Locals[in.Dst].Ent != 0 || f.Locals[in.Dst].Type != sem.TyString {
				continue
			}
			if in.Op == midir.OpCall || in.Op == midir.OpAlias && f.Locals[in.Args[0]].Ent == 0 {
				out[in.Dst] = true
			}
		}
	}
	return out
}

func TestPlanReleasesCallResultsOnEveryPath(t *testing.T) {
	p := mirfixture.Planned(t, liveCallsSrc)
	for _, name := range []string{"main.merged", "main.run"} {
		f := mirfixture.FuncNamed(t, p, name)
		temps := freshStrings(f)
		if len(temps) == 0 {
			t.Fatalf("%s: no String temporaries to follow", name)
		}
		w := &walk{p: p, f: f, temps: temps, seen: map[string]bool{}}
		w.run(0, map[midir.LocalID]bool{})
		for _, e := range w.errs {
			t.Errorf("%s: %s\n%s", name, e, p.Dump())
		}
	}
}
