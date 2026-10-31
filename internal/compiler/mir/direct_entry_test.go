package mir_test

import (
	"testing"

	"kigumi/internal/compiler/mir"
)

func entryOps(f *mir.Func) []mir.Opcode {
	var out []mir.Opcode
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			out = append(out, in.Op)
		}
	}
	return out
}

func TestPlanDirectWritesTheBoxedEntryCrossings(t *testing.T) {
	p := planned(t, crossingSrc)
	take, mk := funcNamed(t, p, "main.take"), funcNamed(t, p, "main.mk")
	if take.Entry == nil || mk.Entry == nil || take.Entry.Wraps != take {
		t.Fatalf("a direct signature has no boxed entry:\n%s", p.Dump())
	}
	if got := entryOps(take.Entry); len(got) != 2 || got[0] != mir.OpUnboxDirect || got[1] != mir.OpCall {
		t.Errorf("take's entry is %v, want an unbox then the call", got)
	}
	if got := entryOps(mk.Entry); len(got) != 2 || got[0] != mir.OpCall || got[1] != mir.OpBoxDirect {
		t.Errorf("mk's entry is %v, want the call then a box", got)
	}
	if err := p.Verify(); err != nil {
		t.Fatal(err)
	}
}

func TestPlanDirectRebuildsAClosureEnvironmentInItsEntry(t *testing.T) {
	p := planned(t, closureEnvSrc)
	for _, f := range p.Funcs {
		if !p.EnvABI(f) {
			continue
		}
		if f.Entry == nil {
			t.Fatalf("%s has no boxed entry", f.Name)
		}
		clo, ok := firstOp(f.Entry, mir.OpClosure)
		call, called := firstOp(f.Entry, mir.OpCallValue)
		if !ok || !called || !f.Entry.IsDirect(clo.Dst) || call.Args[0] != clo.Dst {
			t.Fatalf("%s's entry does not call an unboxed closure it built:\n%s", f.Name, p.Dump())
		}
		return
	}
	t.Fatalf("no closure with an unboxed environment:\n%s", p.Dump())
}

func TestVerifyRejectsADirectSignatureWithoutAnEntry(t *testing.T) {
	p := planned(t, crossingSrc)
	funcNamed(t, p, "main.mk").Entry = nil
	directRule(t, p, "no boxed entry")
}

func TestVerifyRejectsAnEntryTakingAnUnboxedParameter(t *testing.T) {
	p := planned(t, crossingSrc)
	e := funcNamed(t, p, "main.take").Entry
	e.Direct[e.Params[0]] = true
	directRule(t, p, "form")
}

func TestVerifyRejectsAnEntryOfAnotherFunction(t *testing.T) {
	p := planned(t, crossingSrc)
	funcNamed(t, p, "main.mk").Entry.Wraps = funcNamed(t, p, "main.take")
	directRule(t, p, "does not name")
}
