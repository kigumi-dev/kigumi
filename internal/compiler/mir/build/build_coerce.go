package build

import (
	"fmt"

	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// coerce applies the checker's coercion steps to a value.
func (b *builder) coerce(co hir.Coercion, n syntax.NodeID, v midir.LocalID) midir.LocalID {
	for _, s := range co.Coerce {
		switch s.Kind {
		case sem.CoLiteral:
			if b.fn.Locals[v].Type != s.To {
				v = b.emit(midir.Inst{Op: midir.OpUnary, Dst: b.temp(s.To), Str: "cast", Type: s.To, Args: []midir.LocalID{v}, Node: n})
			}
		case sem.CoBorrow:
			v = b.emit(midir.Inst{Op: midir.OpBorrow, Dst: b.temp(s.To), Args: []midir.LocalID{v}, Node: n})
		case sem.CoSome:
			// A source already at the wrapped type means this step ran
			// twice on the same value (a lowering bug), not a legitimate
			// Option[Option[T]]: the checker only wraps a bare T.
			if b.fn.Locals[v].Type == s.To {
				panic(fmt.Sprintf("mir: %s: CoSome applied to a value already of type %s (double coercion)", b.fn.Name, b.r.TypeString(s.To)))
			}
			v = b.emit(midir.Inst{Op: midir.OpVariant, Dst: b.temp(s.To), Ent: b.variantNamed(b.r.Types.OptionEnt(), "Some"), Type: s.To, Args: []midir.LocalID{v}, Node: n})
		case sem.CoOk:
			if b.fn.Locals[v].Type == s.To {
				panic(fmt.Sprintf("mir: %s: CoOk applied to a value already of type %s (double coercion)", b.fn.Name, b.r.TypeString(s.To)))
			}
			v = b.emit(midir.Inst{Op: midir.OpVariant, Dst: b.temp(s.To), Ent: b.variantNamed(b.r.Types.ResultEnt(), "Ok"), Type: s.To, Args: []midir.LocalID{v}, Node: n})
		case sem.CoExistential:
			v = b.emit(midir.Inst{Op: midir.OpBox, Dst: b.temp(s.To), Type: co.CoFrom, Args: []midir.LocalID{v}, Node: n})
		case sem.CoLendExistential:
			v = b.emit(midir.Inst{Op: midir.OpLendBox, Dst: b.temp(s.To), Type: b.r.Types.Node(co.CoFrom).Elem, Args: []midir.LocalID{v}, Node: n})
		case sem.CoCFnPtr:
			v = b.emit(midir.Inst{Op: midir.OpCFnPtr, Dst: b.temp(s.To), Ent: co.CoEnt, Type: s.To, Node: n})
		}
	}
	return v
}
