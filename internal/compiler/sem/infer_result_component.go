package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

func (s methodFieldCallSite) argAt(index int) syntax.NodeID {
	switch {
	case index == selfIndex:
		return s.recvNode
	case index < len(s.args):
		return s.args[index]
	}
	return 0
}

func (r *Result) componentPathType(t TypeID, path []EntityID) (TypeID, bool) {
	for _, field := range path {
		fields, elems, ok := r.containerShape(t)
		if !ok {
			return 0, false
		}
		i := slices.Index(fields, field)
		if i < 0 {
			return 0, false
		}
		t = elems[i]
	}
	return t, true
}
