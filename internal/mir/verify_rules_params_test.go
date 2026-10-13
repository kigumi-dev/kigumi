package mir_test

import (
	"testing"

	"kigumi/internal/mir"
)

const paramRulesSrc = `
fn bump(mut n: Int) -> Int {
    let add = (k: Int) => { n = n + k }
    add(2)
    n
}

fn plain(n: Int) -> Int { n }

let mut v = 1
print "${bump(v)} ${plain(v)}"
`

func TestVerifyParamsOfCapturedMut(t *testing.T) {
	p := planned(t, paramRulesSrc)
	bump := funcNamed(t, p, "main.bump")
	if bump.Locals[bump.Params[0]].Ent == p.R.Fn(bump.Ent).Params[0] {
		t.Fatal("a captured mut parameter arrives as its own local, want a raw value boxed into a cell")
	}
	if err := p.Verify(); err != nil {
		t.Fatalf("a captured mut parameter is rejected: %v", err)
	}
	runRuleCases(t, paramRulesSrc, true, []ruleCase{
		{"parameter of another entity", "main.plain", "calls", "that plain declares there", func(t *testing.T, p *mir.Program, f *mir.Func) {
			f.Locals[f.Params[0]].Ent = f.Ent
		}},
		{"boxed parameter whose cell is another entity", "main.bump", "calls", "that bump declares there", func(t *testing.T, p *mir.Program, f *mir.Func) {
			for _, in := range f.Blocks[0].Insts {
				if in.Op == mir.OpNewCell && in.Args[0] == f.Params[0] {
					f.Locals[in.Dst].Ent = f.Ent
					return
				}
			}
			t.Fatal("no cell boxes the parameter")
		}},
		{"raw parameter that nothing boxes", "main.bump", "calls", "that bump declares there", func(t *testing.T, p *mir.Program, f *mir.Func) {
			for i := range f.Blocks[0].Insts {
				if in := &f.Blocks[0].Insts[i]; in.Op == mir.OpNewCell && in.Args[0] == f.Params[0] {
					in.Args[0] = in.Dst
					return
				}
			}
			t.Fatal("no cell boxes the parameter")
		}},
	})
}
