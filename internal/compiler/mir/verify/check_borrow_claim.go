package verify

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (p *claims) boxedType(f *midir.Func, in midir.Inst) sem.TypeID {
	switch in.Op {
	case midir.OpBoxDirect:
		if in.Fixed {
			return 0
		}
		return in.Type
	case midir.OpBorrow:
		return p.BoxedLend(f, in)
	}
	return 0
}
