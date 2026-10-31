package mir_test

import (
	"testing"

	"kigumi/internal/compiler/mir"
)

const reassignedParameterSrc = `
fn replace(mut h: Int?) -> Int {
    h = Some(3)
    match h { Some(v) -> v, None -> 0 }
}
fn fromValue(mut h: Int?, value: Int?) -> Int {
    h = value
    match h { Some(v) -> v, None -> 0 }
}
fn second(ignored: Int, mut h: Int?) -> Int {
    h = Some(8)
    match h { Some(v) -> v, None -> 0 }
}
fn walk(mut h: Int?) -> Int {
    let mut total = 0
    for h is Some(v) {
        total += v
        h = if v > 0 { Some(v - 1) } else { None }
    }
    total
}
let _ = replace(None)
let _ = fromValue(None, Some(7))
let _ = second(1, None)
let _ = walk(Some(4))
`

func TestVerifyVariantsReassignedParameter(t *testing.T) {
	p := mir.Build(checkSourceMir(t, reassignedParameterSrc))
	if p.HasErrors() {
		t.Fatal(p.Render())
	}
	if err := p.Verify(); err != nil {
		t.Fatalf("verify: %v\n%s", err, userDump(p))
	}
	if err := p.Apply(mir.Transform{Name: "plan-rc", Run: mir.PlanRC}, mir.Transform{Name: "plan-direct", Run: mir.PlanDirect}); err != nil {
		t.Fatalf("planned: %v\n%s", err, userDump(p))
	}
}

func TestVerifyVariantsReassignedParameterWrongEdge(t *testing.T) {
	runRuleCases(t, reassignedParameterSrc, false, []ruleCase{
		{"payload on the wrong parameter edge", "main.fromValue", "variants", "which may hold None with 0", func(t *testing.T, p *mir.Program, f *mir.Func) {
			for i := range f.Blocks {
				term := &f.Blocks[i].Term
				if term.Op == mir.TermBranch {
					term.Targets[0], term.Targets[1] = term.Targets[1], term.Targets[0]
					return
				}
			}
			t.Fatal("no match branch")
		}},
	})
}
