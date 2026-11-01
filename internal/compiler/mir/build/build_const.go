package build

import (
	"fmt"
	"math/big"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// constArgs lowers the const-parameter values a call passes after its
// witness arguments, mirroring witnessArgs for WitnessArg.
func (b *builder) constArgs(args []sem.ConstArg, n syntax.NodeID) []midir.LocalID {
	var out []midir.LocalID
	for _, a := range args {
		out = append(out, b.constArg(a, n))
	}
	return out
}

func (b *builder) constArg(a sem.ConstArg, n syntax.NodeID) midir.LocalID {
	switch a.Kind {
	case sem.ConstArgLocal:
		l := b.local(a.Local)
		return b.emit(midir.Inst{Op: midir.OpCopy, Dst: b.temp(b.fn.Locals[l].Type), Args: []midir.LocalID{l}})
	case sem.ConstArgLit:
		if a.Type == sem.TyBool {
			return b.emit(midir.Inst{Op: midir.OpConst, Dst: b.temp(a.Type), Lit: sem.Literal{Kind: sem.LitBool, Bool: a.Value != 0}, Type: a.Type, Node: n})
		}
		return b.emit(midir.Inst{Op: midir.OpConst, Dst: b.temp(a.Type), Lit: sem.Literal{Kind: sem.LitInt, Int: big.NewInt(a.Value)}, Type: a.Type, Node: n})
	}
	// Zero Kind means sem accepted a call it could not build a const
	// argument for: a checker bug, not a user error.
	panic(fmt.Sprintf("mir: %s: empty ConstArg for an accepted call", b.fn.Name))
}
