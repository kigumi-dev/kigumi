package build

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/syntax"
)

// failWith wraps the error in Err of the function's Result type and
// returns it after unwinding every scope, parameters included, with the
// error cleanups active.
func (b *builder) failWith(n syntax.NodeID, e midir.LocalID) {
	b.closeTaskScope(n)
	b.unwindTo(0, true)
	ret := b.ret
	if ret == 0 {
		ret = b.fn.Ret
	}
	err := b.emit(midir.Inst{Op: midir.OpVariant, Dst: b.temp(ret), Ent: b.variantNamed(b.r.Types.ResultEnt(), "Err"), Type: ret, Args: []midir.LocalID{e}, Node: n})
	b.returnValue(err)
}
