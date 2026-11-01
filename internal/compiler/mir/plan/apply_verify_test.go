package plan_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	mirplan "kigumi/internal/compiler/mir/plan"
	"kigumi/internal/devkit/mirfixture"
)

func TestApplyVerifiesOnceUnlessAskedPerPass(t *testing.T) {
	mirplan.SetVerifyEachPass(false)
	t.Cleanup(func() { mirplan.SetVerifyEachPass(true) })
	breakEntry := mirplan.Transform{Name: "break", Run: func(p *mir.Program) error {
		p.Entry.Blocks[0].Term = midir.Term{}
		return nil
	}}
	keep := mirplan.Transform{Name: "keep", Run: func(*mir.Program) error { return nil }}
	for _, each := range []bool{false, true} {
		p := mirfixture.Built(t, "print \"x\"\n")
		p.VerifyEach = each
		want := "after passes: "
		if each {
			want = "after break: "
		}
		if err := mirplan.Apply(p, breakEntry, keep); err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("VerifyEach=%v: got %v, want an error starting %q", each, err, want)
		}
	}
}
