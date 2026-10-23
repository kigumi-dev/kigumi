package mir

import (
	"slices"

	"kigumi/internal/diag"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// touch reports whether in touches owner, and its field path if so.
// OpField/OpSetField/OpBorrow resolve the full projection so a
// field-chain borrow is checked by field, not by the temp it borrows.
func (c *borrowCheck) touch(in Inst, owner LocalID) (touched bool, path []int, exact bool) {
	switch in.Op {
	case OpField, OpPayload:
		if c.chainedOnly(in.Dst) {
			return false, nil, true
		}
		if root, p, ex, ok := c.projectionPlace(in.Dst); ok && root == owner {
			return true, p, ex
		}
	case OpSetField, OpFieldMove:
		if root, p, ex, ok := c.projectionPlace(in.Args[0]); ok && root == owner {
			if ex {
				p = append(p, in.Index)
			}
			return true, p, ex
		}
	case OpBorrow:
		if root, p, ex, ok := c.projectionPlace(in.Args[0]); ok && root == owner {
			return true, p, ex
		}
	case OpSetIndex:
		if root, p, ex, ok := c.projectionPlace(in.Args[0]); ok && root == owner {
			return true, p, ex
		}
		return touchesLocal(in, owner), nil, true
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
				if d := c.definition(in.Args[0]); d.Op == OpField && ex {
					p = append(p, d.Index)
				}
				return true, p, ex
			}
		}
		fallthrough
	default:
		return touchesLocal(in, owner), nil, true
	}
	return false, nil, true
}

func touchesLocal(in Inst, l LocalID) bool {
	return in.Dst == l && in.Op != OpDrop || slices.Contains(in.Args, l)
}

// reportKey dedups one diagnostic at one node; local is needed too since an unnamed
// temporary's synthesized drop falls back to sharing one node with others like it.
type reportKey struct {
	node  syntax.NodeID
	code  string
	local LocalID
}

func (c *borrowCheck) report(n syntax.NodeID, local LocalID, code, msg string) {
	c.reportNoted(n, local, code, msg, 0, "")
}

func (c *borrowCheck) reportNoted(n syntax.NodeID, local LocalID, code, msg string, note syntax.NodeID, noteMsg string) {
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
	t := c.p.R.Tree(c.f.File)
	if at, seen := c.reported[key]; seen {
		if note != 0 {
			c.keepNearestNote(at, diag.At(t.File, t.Span(note)))
		}
		return
	}
	d := diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(n)), Msg: msg, Code: diagCode(code)}
	if note != 0 && note != n {
		d = d.WithNote(diag.At(t.File, t.Span(note)), noteMsg)
	}
	if code == "borrow-conflict" {
		d = d.WithHelp(sem.HelpOf(d.Code))
	}
	c.reported[key] = len(c.f.Diags)
	c.f.Diags = append(c.f.Diags, d)
}

// keepNearestNote keeps the latest borrow before the conflict, since holders are visited in map order.
func (c *borrowCheck) keepNearestNote(at int, loc diag.Location) {
	d := &c.f.Diags[at]
	if len(d.Notes) == 0 {
		return
	}
	use, cur := d.Loc.Span.Start, d.Notes[0].Loc.Span.Start
	if loc.Span.Start < use && (cur >= use || loc.Span.Start > cur) {
		d.Notes[0].Loc = loc
	}
}
