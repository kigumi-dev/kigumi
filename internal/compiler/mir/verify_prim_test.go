package mir_test

import (
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

func primitiveOf(t *testing.T, p *mir.Program, arg, result sem.TypeID) sem.EntityID {
	t.Helper()
	for i := range p.R.Entities {
		id := sem.EntityID(i)
		if !ownquery.IsStdPrimitive(p.R, id) {
			continue
		}
		if sig := p.R.Types.Node(p.R.Fn(id).Sig); len(sig.Args) == 1 && sig.Args[0] == arg && sig.Elem == result {
			return id
		}
	}
	t.Fatalf("no std primitive of %s -> %s", p.R.TypeString(arg), p.R.TypeString(result))
	return 0
}

func TestVerifyPrimitiveOperandKinds(t *testing.T) {
	p := program(t)
	sqrt := primitiveOf(t, p, sem.TyF64, sem.TyF64)
	locals := []midir.Local{{Name: "t0", Type: sem.TyF64}, {Name: "t1", Type: sem.TyString}, {Name: "t2", Type: sem.TyI64}}
	call := func(arg, dst midir.LocalID) *midir.Func {
		return fn("prim", locals[dst].Type, locals, midir.Block{Insts: []midir.Inst{{Op: midir.OpCall, Ent: sqrt, Dst: dst, Args: []midir.LocalID{arg}}}, Term: ret(dst)})
	}
	expectRule(t, p, call(1, 0), "types", "call of sqrt: argument 0 wants scalar Float, got object String")
	expectRule(t, p, call(2, 0), "types", "argument 0 wants scalar Float, got scalar Int")
	expectRule(t, p, call(0, 1), "types", "result wants scalar Float, got object String")
}
