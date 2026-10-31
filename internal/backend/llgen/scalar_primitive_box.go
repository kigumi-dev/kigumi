package llgen

import (
	"fmt"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
)

func (e *emitter) primitiveScalarBox(f *mir.Func, in mir.Inst, at siteAt) (mir.OperationBox, bool) {
	op, recorded := e.boxes.primitives[at]
	actual, ok := in.StdOp()
	if !recorded || !ok || actual != op {
		return mir.OperationBox{}, false
	}
	src := mir.Registry.Ops[op].ScalarBox
	t := f.Locals[in.Args[src.Argument]].Type
	for {
		t = e.r.UnderlyingRepr(t)
		if e.r.Types.Kind(t) != sem.KRef {
			break
		}
		t = e.r.Types.Node(t).Elem
	}
	args := e.r.Types.Node(t).Args
	if src.TypeArg >= len(args) {
		panic(fmt.Sprintf("llgen: %s scalar-box needs type argument %d of argument %d", mir.StdOpNames[op], src.TypeArg, src.Argument))
	}
	t = args[src.TypeArg]
	if !e.r.ScalarBoxes(t) {
		return mir.OperationBox{}, false
	}
	return mir.OperationBox{Block: at.blk, Inst: at.pc, Type: t}, true
}
