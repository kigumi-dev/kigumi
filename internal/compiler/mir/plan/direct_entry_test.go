package plan_test

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	mirverify "kigumi/internal/compiler/mir/verify"
	"kigumi/internal/devkit/mirfixture"
)

func entryOps(f *midir.Func) []midir.Opcode {
	var out []midir.Opcode
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			out = append(out, in.Op)
		}
	}
	return out
}

func TestPlanDirectWritesTheBoxedEntryCrossings(t *testing.T) {
	p := mirfixture.Planned(t, mirfixture.CrossingSrc)
	take, mk := mirfixture.FuncNamed(t, p, "main.take"), mirfixture.FuncNamed(t, p, "main.mk")
	if take.Entry == nil || mk.Entry == nil || take.Entry.Wraps != take {
		t.Fatalf("a direct signature has no boxed entry:\n%s", p.Dump())
	}
	if got := entryOps(take.Entry); len(got) != 2 || got[0] != midir.OpUnboxDirect || got[1] != midir.OpCall {
		t.Errorf("take's entry is %v, want an unbox then the call", got)
	}
	if got := entryOps(mk.Entry); len(got) != 2 || got[0] != midir.OpCall || got[1] != midir.OpBoxDirect {
		t.Errorf("mk's entry is %v, want the call then a box", got)
	}
	if err := mirverify.Program(p); err != nil {
		t.Fatal(err)
	}
}

func TestPlanDirectRebuildsAClosureEnvironmentInItsEntry(t *testing.T) {
	p := mirfixture.Planned(t, mirfixture.ClosureEnvSrc)
	for _, f := range p.Funcs {
		if !p.EnvABI(f) {
			continue
		}
		if f.Entry == nil {
			t.Fatalf("%s has no boxed entry", f.Name)
		}
		clo, ok := mirfixture.FirstOp(f.Entry, midir.OpClosure)
		call, called := mirfixture.FirstOp(f.Entry, midir.OpCallValue)
		if !ok || !called || !f.Entry.IsDirect(clo.Dst) || call.Args[0] != clo.Dst {
			t.Fatalf("%s's entry does not call an unboxed closure it built:\n%s", f.Name, p.Dump())
		}
		return
	}
	t.Fatalf("no closure with an unboxed environment:\n%s", p.Dump())
}

func TestVerifyRejectsADirectSignatureWithoutAnEntry(t *testing.T) {
	p := mirfixture.Planned(t, mirfixture.CrossingSrc)
	mirfixture.FuncNamed(t, p, "main.mk").Entry = nil
	mirfixture.DirectRule(t, p, "no boxed entry")
}

func TestVerifyRejectsAnEntryTakingAnUnboxedParameter(t *testing.T) {
	p := mirfixture.Planned(t, mirfixture.CrossingSrc)
	e := mirfixture.FuncNamed(t, p, "main.take").Entry
	e.Direct[e.Params[0]] = true
	mirfixture.DirectRule(t, p, "form")
}

func TestVerifyRejectsAnEntryOfAnotherFunction(t *testing.T) {
	p := mirfixture.Planned(t, mirfixture.CrossingSrc)
	mirfixture.FuncNamed(t, p, "main.mk").Entry.Wraps = mirfixture.FuncNamed(t, p, "main.take")
	mirfixture.DirectRule(t, p, "does not name")
}
