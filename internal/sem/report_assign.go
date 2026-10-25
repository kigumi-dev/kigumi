package sem

import (
	"strings"

	"kigumi/internal/syntax"
)

// emits E701/E702 for a mutable-place violation, walking outside-in like recvMutabilityFixes (E515)
// to blame the link nearest the point of use.
func (c *checker) reportImmutable(lhs syntax.NodeID) {
	node := c.t.View(lhs)
	switch node.Kind() {
	case syntax.Ident:
		if root := c.rootOf(lhs); root != 0 {
			name := node.Tok().Text()
			if c.rebindsSharedRef(root) {
				c.errRebindSharedRef(lhs, name)
				return
			}
			fix := "declare `let mut " + name + "`"
			if e := c.r.Entities[root]; e.File == c.f && e.Tok != 0 {
				c.errFix(lhs, c.r.insertAt(c.f, e.Tok, "Declare `mut "+name+"`", "mut "), cAssignImmutable, name, fix)
				return
			}
			c.errAt(lhs, cAssignImmutable, name, fix)
			return
		}
	case syntax.MemberExpr:
		if fld := c.info.Uses[lhs]; fld != 0 {
			switch c.r.Entities[fld].Kind {
			case EntField:
				if fe := c.r.Entities[fld]; fe.Flags&EfMut == 0 {
					c.reportImmutableField(lhs, fe.Name)
					return
				}
				c.reportImmutableBase(node.Base().ID())
				return
			case EntType:
				// `.value`: same storage as the receiver, just retyped (mirrors buildPlace).
				c.reportImmutableBase(node.Base().ID())
				return
			}
		}
	case syntax.BracketExpr:
		if c.isPlace(lhs) {
			c.reportImmutableBase(node.Base().ID())
			return
		}
	}
	c.errAt(lhs, cAssignNotPlace)
}

// reportImmutableBase is reportImmutable for the base of a written place, which a write goes through.
func (c *checker) reportImmutableBase(base syntax.NodeID) {
	if c.t.Kind(base) == syntax.Ident {
		if fix, ok := c.sharedRefFix(c.rootOf(base)); ok {
			c.errAt(base, cAssignImmutable, c.t.View(base).Tok().Text(), fix)
			return
		}
	}
	c.reportImmutable(base)
}

func (c *checker) reportImmutableField(lhs syntax.NodeID, name string) {
	fix, tok, file := c.assignImmutableFix(lhs)
	if file != 0 && file == c.f && tok != 0 {
		c.errFix(lhs, c.r.insertAt(c.f, tok, "Declare `mut "+name+"`", "mut "), cAssignImmutableField, name, fix)
		return
	}
	c.errAt(lhs, cAssignImmutableField, name, fix)
}

// mirrors recvMutabilityFix (E515): an insert-before token is offered only for a single link.
func (c *checker) assignImmutableFix(n syntax.NodeID) (fix string, tok uint32, file FileID) {
	fixes, tok, file := c.assignImmutableFixes(n)
	switch len(fixes) {
	case 0:
		return "make it mutable", 0, 0
	case 1:
		return fixes[0], tok, file
	default:
		return strings.Join(fixes, ", and "), 0, 0
	}
}

// walks n's place chain outside-in for every non-mutable link (nearest point of use first); a
// chain can carry both a non-mut field and an immutable root, so it must not stop at the first.
func (c *checker) assignImmutableFixes(n syntax.NodeID) (fixes []string, tok uint32, file FileID) {
	switch c.t.Kind(n) {
	case syntax.Paren, syntax.BracketExpr:
		return c.assignImmutableFixes(c.t.View(n).Base().ID())
	case syntax.MemberExpr:
		rest, rtok, rfile := c.assignImmutableFixes(c.t.View(n).Base().ID())
		if fld := c.info.Uses[n]; fld != 0 && c.r.Entities[fld].Kind == EntField {
			if fe := c.r.Entities[fld]; fe.Flags&EfMut == 0 {
				fix := "declare `mut " + fe.Name + "`"
				return append([]string{fix}, rest...), fe.Tok, fe.File
			}
		}
		return rest, rtok, rfile
	case syntax.Ident:
		root := c.rootOf(n)
		if root == 0 || c.isMutablePlace(n) {
			return nil, 0, 0
		}
		if fix, ok := c.sharedRefFix(root); ok {
			return []string{fix}, 0, 0
		}
		e := &c.r.Entities[root]
		return []string{"declare `let mut " + e.Name + "`"}, e.Tok, e.File
	}
	return nil, 0, 0
}

// sharedRefFix is the fix for writing through a binding of a shared reference, which `let mut` would not allow.
func (c *checker) sharedRefFix(root EntityID) (string, bool) {
	if root == 0 {
		return "", false
	}
	e := &c.r.Entities[root]
	if c.r.Types.Kind(e.Type) != KRef || c.r.Types.RefMut(e.Type) {
		return "", false
	}
	return "annotate `" + e.Name + "` as `&mut " + c.r.TypeString(c.r.Types.Node(e.Type).Elem) + "`", true
}

func (c *checker) rebindsSharedRef(root EntityID) bool {
	e := &c.r.Entities[root]
	t := c.vars.resolve(e.Type)
	return e.Flags&EfMut != 0 && c.r.Types.Kind(t) == KRef && !c.r.Types.RefMut(t)
}

// errRebindSharedRef keeps E701 but states the real reason, since the binding is `let mut`.
func (c *checker) errRebindSharedRef(lhs syntax.NodeID, name string) {
	d, ok := c.r.diagAt(c.f, lhs, cAssignImmutable, name, "")
	if !ok {
		return
	}
	d.Msg = "`" + name + "` holds a shared reference and cannot be reassigned"
	d.Help = []string{"shadow it with a new `let " + name + "` to refer to another value"}
	c.r.emitDiag(c.f, d)
}
