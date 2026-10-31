package mir

import (
	"slices"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
)

// touch reports whether in touches owner, and its field path if so.
// OpField/OpSetField/OpBorrow resolve the full projection so a
// field-chain borrow is checked by field, not by the temp it borrows.
func (c *borrowCheck) touch(in Inst, owner LocalID) (touched bool, path stepPath, exact bool) {
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
	// Reading a lent receiver's write-back cell, or the cell a local lives in, stands for the
	// local itself, so a field chain through it resolves like a plain one.
	case OpCellGet:
		if !c.f.Locals[in.Args[0]].Borrowed && !c.placeCell(in.Args[0]) {
			return in.Dst == owner || in.Args[0] == owner, nil, true
		}
		if c.chainedOnly(in.Dst) {
			return false, nil, true
		}
		if root, p, ex, ok := c.projectionPlace(in.Dst); ok && root == owner {
			return true, p, ex
		}
		return false, nil, true
	// A closure literal shares a cell it captures without reading it; running the closure is the touch (cellRunHit).
	case OpClosure:
		return touchesLocal(in, owner) && !c.f.Locals[owner].Cell, nil, true
	// Dropping a cell the caller handed in releases only this body's handle; the frame owning the cell keeps the content.
	case OpDrop:
		return in.Args[0] == owner && !c.paramCell(owner), nil, true
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

// An instruction with no result keeps Dst at 0, which is also the first parameter's local.
func touchesLocal(in Inst, l LocalID) bool {
	return in.Op.Yields() && in.Dst == l && in.Op != OpDrop || slices.Contains(in.Args, l)
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
	c.reportHelped(n, local, code, msg, note, noteMsg, "")
}

// reportHelped is reportNoted with a help line of its own in place of the catalog's, when help is not empty.
func (c *borrowCheck) reportHelped(n syntax.NodeID, local LocalID, code, msg string, note syntax.NodeID, noteMsg, help string) {
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
		c.keepCommonHelp(at, code, help)
		return
	}
	d := diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(n)), Msg: msg, Code: diagCode(code)}
	if note != 0 && note != n {
		d = d.WithNote(diag.At(t.File, t.Span(note)), noteMsg)
	}
	if help == "" && code == "borrow-conflict" {
		help = sem.HelpOf(d.Code)
	}
	if help != "" {
		d = d.WithHelp(help)
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

// keepCommonHelp falls back to the catalog's help when holders, visited in map order, would give one conflict different helps.
func (c *borrowCheck) keepCommonHelp(at int, code, help string) {
	d := &c.f.Diags[at]
	if code != "borrow-conflict" || len(d.Help) != 1 {
		return
	}
	if help == "" {
		help = sem.HelpOf(d.Code)
	}
	if d.Help[0] != help {
		d.Help = []string{sem.HelpOf(d.Code)}
	}
}
