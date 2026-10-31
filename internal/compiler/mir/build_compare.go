package mir

import (
	"kigumi/internal/compiler/hir"
	"kigumi/internal/compiler/sem"
)

const (
	builtinCmpUnordered = "cmp.unordered"
	builtinCmpGuard     = "cmp.guard"
)

// A float witness with a NaN operand makes the comparison false (true for `!=`), as a concrete one does.
func (b *builder) witnessCompare(e *hir.Expr, mark int) LocalID {
	n := e.Node
	recv, rest := b.methodArgs(e)
	ret, ordering := sem.TyBool, b.r.PackageMember(b.r.PackageByPath("std/prelude"), "Ordering")
	ordered := e.Cmp != "==" && e.Cmp != "!="
	if ordered {
		ret = b.r.Types.Named(ordering, nil)
	}
	unordered := b.emit(Inst{Op: OpBuiltin, Dst: b.temp(sem.TyBool), Str: builtinCmpUnordered, Args: []LocalID{b.witnessLocal(e.Witness), recv, rest[0]}, Node: n})
	args := append([]LocalID{b.witnessLocal(e.Witness), recv}, rest...)
	args = append(args, b.ctxArgs(e)...)
	res := b.emit(Inst{Op: OpCallValue, Dst: b.temp(ret), Ent: e.Ent, Args: args, Node: n})
	b.applyArgFixups(mark)
	if ordered {
		variant := "Less"
		if e.Cmp == ">" || e.Cmp == "<=" {
			variant = "Greater"
		}
		res = b.isExpr(&hir.Pat{Kind: hir.PatCtor, Ent: b.variantNamed(ordering, variant), Type: ret, Node: n}, res, false)
		if e.Cmp == "<=" || e.Cmp == ">=" {
			res = b.emit(Inst{Op: OpUnary, Dst: b.temp(sem.TyBool), Str: "!", Args: []LocalID{res}, Node: n})
		}
	}
	res = b.emit(Inst{Op: OpBuiltin, Dst: b.temp(sem.TyBool), Str: builtinCmpGuard, Args: []LocalID{res, unordered}, Node: n})
	if e.Cmp == "!=" {
		res = b.emit(Inst{Op: OpUnary, Dst: b.temp(sem.TyBool), Str: "!", Args: []LocalID{res}, Node: n})
	}
	return res
}
