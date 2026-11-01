package verify_test

import (
	"testing"

	"kigumi/internal/compiler/mir"
	mirbuild "kigumi/internal/compiler/mir/build"
	midir "kigumi/internal/compiler/mir/ir"
	mirplan "kigumi/internal/compiler/mir/plan"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/devkit/mirfixture"
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
	p := mirbuild.Program(mirfixture.CheckSourceMir(t, reassignedParameterSrc))
	if p.HasErrors() {
		t.Fatal(p.Render())
	}
	if err := mirverify.Program(p); err != nil {
		t.Fatalf("verify: %v\n%s", err, mirfixture.UserDump(p))
	}
	if err := mirplan.Apply(p, mirplan.Transform{Name: "plan-rc", Run: mirplan.PlanRC}, mirplan.Transform{Name: "plan-direct", Run: mirplan.PlanDirect}); err != nil {
		t.Fatalf("planned: %v\n%s", err, mirfixture.UserDump(p))
	}
}

func TestVerifyVariantsReassignedParameterWrongEdge(t *testing.T) {
	runRuleCases(t, reassignedParameterSrc, false, []ruleCase{
		{"payload on the wrong parameter edge", "main.fromValue", "variants", "which may hold None with 0", func(t *testing.T, p *mir.Program, f *midir.Func) {
			for i := range f.Blocks {
				term := &f.Blocks[i].Term
				if term.Op == midir.TermBranch {
					term.Targets[0], term.Targets[1] = term.Targets[1], term.Targets[0]
					return
				}
			}
			t.Fatal("no match branch")
		}},
	})
}
