package mir

import (
	"kigumi/internal/diag"
	"kigumi/internal/syntax"
)

// touch reports whether in touches owner, and its field path if so.
// OpField/OpSetField/OpBorrow resolve the full projection so a
// field-chain borrow is checked by field, not by the temp it borrows.
func (c *borrowCheck) touch(in Inst, owner LocalID) (touched bool, path []int, exact bool) {
	switch in.Op {
	case OpField:
		if c.chainedOnly(in.Dst) {
			return false, nil, true
		}
		if root, p, ex, ok := c.projectionPlace(in.Dst); ok && root == owner {
			return true, p, ex
		}
	case OpSetField, OpFieldMove:
		if root, p, ex, ok := c.projectionPlace(in.Args[0]); ok && root == owner {
			return true, append(p, in.Index), ex
		}
	case OpBorrow:
		if root, p, ex, ok := c.projectionPlace(in.Args[0]); ok && root == owner {
			return true, p, ex
		}
	// Reading a lent receiver's write-back cell stands for the receiver
	// itself, so a field chain through it resolves like a plain one.
	case OpCellGet:
		if !c.f.Locals[in.Args[0]].Borrowed {
			return in.Dst == owner || in.Args[0] == owner, nil, true
		}
		if c.chainedOnly(in.Dst) {
			return false, nil, true
		}
		if root, p, ex, ok := c.projectionPlace(in.Dst); ok && root == owner {
			return true, p, ex
		}
		return false, nil, true
	case OpCopy:
		if c.takesPayload(in) && c.f.Locals[in.Args[0]].Ent != 0 {
			if in.Args[0] == owner {
				return true, nil, true
			}
		} else if c.takesPayload(in) {
			if root, p, ex, ok := c.projectionPlace(c.definition(in.Args[0]).Args[0]); ok && root == owner {
				if d := c.definition(in.Args[0]); d.Op == OpField {
					p = append(p, d.Index)
				}
				return true, p, ex
			}
		}
		fallthrough
	default:
		if in.Dst == owner && in.Op != OpDrop {
			return true, nil, true
		}
		for _, a := range in.Args {
			if a == owner {
				return true, nil, true
			}
		}
	}
	return false, nil, true
}

// reportKey dedups one diagnostic at one node; local is needed too since an unnamed
// temporary's synthesized drop falls back to sharing one node with others like it.
type reportKey struct {
	node  syntax.NodeID
	code  string
	local LocalID
}

func (c *borrowCheck) report(n syntax.NodeID, local LocalID, code, msg string) {
	if c.quiet {
		return
	}
	if n == 0 {
		n = c.p.R.Entity(c.f.Ent).Node
	}
	if n == 0 {
		return
	}
	key := reportKey{n, code, local}
	if c.reported[key] {
		return
	}
	c.reported[key] = true
	t := c.p.R.Tree(c.f.File)
	c.f.Diags = append(c.f.Diags, diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(n)), Msg: msg, Code: diagCode(code)})
}
