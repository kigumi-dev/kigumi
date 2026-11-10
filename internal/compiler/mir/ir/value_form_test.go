package ir

import "testing"

func TestValueForm(t *testing.T) {
	for _, f := range []ValueForm{FormClosure, FormMethod, FormWitness, FormApplied} {
		if got := (&Inst{Op: OpClosure, Str: f.Tag()}).Form(); got != f {
			t.Errorf("tag %q reads back as %d, want %d", f.Tag(), got, f)
		}
	}
	if got := (&Inst{Op: OpClosure, Str: "bound"}).Form(); got != FormUnknown {
		t.Errorf("an unlisted tag is form %d, want FormUnknown", got)
	}
	if got := (&Inst{Op: OpBind, Str: "method"}).Form(); got != FormNone {
		t.Errorf("a non-closure is form %d, want FormNone", got)
	}
	for _, f := range []ValueForm{FormNone, FormUnknown} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("form %d has a tag", f)
				}
			}()
			f.Tag()
		}()
	}
}
