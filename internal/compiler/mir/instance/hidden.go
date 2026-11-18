package instance

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// Bound reports whether fn, reached through a descriptor, needs the instantiation: it takes a hidden const
// argument, or specialize made it a body per instantiation.
func Bound(r *sem.Result, b *Bodies, fn sem.EntityID) bool {
	return len(r.Fn(fn).ConstParams) > 0 || b.Specialized(fn)
}

// Hidden is what a descriptor call of fn passes after the ordinary arguments: a placeholder per witness slot,
// which sem keeps such a body from reading (E963, E2257), then the const arguments.
type Hidden struct {
	Witnesses int
	Consts    []sem.ConstArg
}

func HiddenArgs(r *sem.Result, fn sem.EntityID, inst sem.TypeID) (Hidden, bool) {
	consts, ok := r.InstanceConstArgs(fn, inst)
	if !ok {
		return Hidden{}, false
	}
	return Hidden{Witnesses: len(r.Fn(fn).Witnesses), Consts: consts}, true
}

// ConstInst is the instruction a hidden const argument is lowered to at a call site.
func ConstInst(a sem.ConstArg) midir.Inst {
	return midir.Inst{Op: midir.OpConst, Lit: a.Literal(), Type: a.Type}
}
