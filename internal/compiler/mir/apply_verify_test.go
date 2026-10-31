package mir_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
)

func TestApplyVerifiesOnceUnlessAskedPerPass(t *testing.T) {
	mir.SetVerifyEachPass(false)
	t.Cleanup(func() { mir.SetVerifyEachPass(true) })
	breakEntry := mir.Transform{Name: "break", Run: func(p *mir.Program) error {
		p.Entry.Blocks[0].Term = mir.Term{}
		return nil
	}}
	keep := mir.Transform{Name: "keep", Run: func(*mir.Program) error { return nil }}
	for _, each := range []bool{false, true} {
		p := built(t, "print \"x\"\n")
		p.VerifyEach = each
		want := "after passes: "
		if each {
			want = "after break: "
		}
		if err := p.Apply(breakEntry, keep); err == nil || !strings.HasPrefix(err.Error(), want) {
			t.Errorf("VerifyEach=%v: got %v, want an error starting %q", each, err, want)
		}
	}
}
