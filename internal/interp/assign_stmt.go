package interp

import (
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

func (fr *frame) assignStmt(n syntax.NodeID) *ctrl {
	node := fr.t.Nodes[n]
	lhs, rhs := syntax.NodeID(node.Lhs), syntax.NodeID(node.Rhs)
	if fr.t.Kind(lhs) == syntax.Ident && fr.t.TokText(fr.t.Nodes[lhs].Tok) == "_" {
		v, c := fr.expr(rhs)
		if c == nil && fr.in.moveOnly(v) {
			// The discarded value may wrap a resource (an Option out of a map).
			fr.markMoved(rhs, v)
			fr.dropDeep(v)
		}
		return c
	}
	if call, ok := fr.info.Calls[n]; ok && (call.Kind == sem.CallBuiltinOp || call.Kind == sem.CallOperator) {
		cell, path, err := fr.resolvePlace(lhs)
		if err != nil {
			return err
		}
		cur := walkPath(cell.V, path)
		r, c := fr.expr(rhs)
		if c != nil {
			return c
		}
		var out Value
		if call.Kind == sem.CallOperator {
			out, c = fr.callUser(call.Callee, n, []Value{cur, r}, nil)
		} else {
			out = fr.binaryOp(n, call.Op, cur, r)
		}
		if c != nil {
			return c
		}
		fr.writePlace(cell, path, out)
		return nil
	}
	cell, path, err := fr.resolvePlace(lhs)
	if err != nil {
		return err
	}
	v, c := fr.expr(rhs)
	if c != nil {
		return c
	}
	fr.writePlace(cell, path, fr.transfer(rhs, v))
	return nil
}
