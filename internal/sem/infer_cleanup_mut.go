package sem

import (
	"kigumi/internal/syntax"
)

// Bodiless std methods that can replace the receiver's content; their declarations look like `push`, so they are named here.
var stdReplacesReceiver = map[string]bool{
	"alloc.Box.set": true, "alloc.Box.withMut": true,
	"array.Array.withMut": true, "prelude.FixedArray.withMut": true,
}

func (c *checker) reserveMutBorrow(at, operand syntax.NodeID) {
	if len(c.reserved) == 0 {
		return
	}
	p, ok := c.buildPlace(operand)
	if !ok {
		return
	}
	if i, held := c.reservedWithin(p.Root, p.Fields); held && !c.mutBorrowKeeps(at, c.reserved[p.Root][i], p.Fields) {
		c.reservedError(at, cCleanupReservedBorrowMut, c.reserved[p.Root][i])
	}
}

func (c *checker) reserveMutRecv(base syntax.NodeID, m EntityID) {
	if len(c.reserved) == 0 {
		return
	}
	p, ok := c.buildPlace(base)
	if !ok {
		return
	}
	i, held := c.reservedWithin(p.Root, p.Fields)
	if !held {
		return
	}
	r := c.reserved[p.Root][i]
	field := ""
	if rel := r.fields[len(p.Fields):]; len(rel) > 0 {
		field = c.r.Entities[rel[0]].Name
	}
	if !c.mutRecvKeepsPlace(m, field) {
		c.reservedError(base, cCleanupReservedBorrowMut, r)
	}
}

func (c *checker) mutRecvKeepsPlace(m EntityID, field string) bool {
	s := selfScan{c: c, owner: c.r.Fn(m).Owner, field: field, root: "self", seen: map[selfScanKey]bool{}}
	return !s.mayReplace(m)
}

func (r *Result) bodilessKeepsReceiver(m EntityID) bool {
	e := &r.Entities[m]
	return e.Flags&EfStd != 0 && r.Entities[e.Parent].Kind != EntInterface && !stdReplacesReceiver[r.stdKey(m)]
}

type selfScan struct {
	c     *checker
	owner EntityID
	field string
	root  string
	fn    EntityID
	seen  map[selfScanKey]bool
}

type selfScanKey struct {
	m     EntityID
	field string
}

func (s *selfScan) mayReplace(m EntityID) bool {
	if s.seen[selfScanKey{m, s.field}] {
		return false
	}
	s.seen[selfScanKey{m, s.field}] = true
	info := s.c.r.Fn(m)
	if info.Body == 0 {
		return !s.c.r.bodilessKeepsReceiver(m)
	}
	if info.Owner == 0 {
		return true
	}
	sub := *s
	sub.root, sub.fn = "self", m
	return sub.scan(s.c.r.tree(s.c.r.Entities[m].File), info.Body)
}

func (s *selfScan) scan(t *syntax.Tree, n syntax.NodeID) bool {
	v := t.View(n)
	switch v.Kind() {
	case syntax.Ident:
		return v.Tok().Text() == s.root
	case syntax.CallExpr:
		if handled, replaced := s.scanCall(t, v); handled {
			return replaced
		}
	case syntax.MemberExpr:
		if base := v.AsMember().Base().ID(); t.Kind(base) == syntax.Ident && t.View(base).Tok().Text() == s.root {
			return s.selfMember(v.Tok().Text())
		}
		if s.field == "" && s.fieldMethodMayReplace(t, v) {
			return true
		}
	case syntax.AssignStmt:
		if rootedAtSelf(t, v.AsAssign().Target().ID(), s.root, false) || s.field == "" && s.ownsFieldPlace(t, v.AsAssign().Target().ID()) {
			return true
		}
	case syntax.BorrowExpr:
		if b := v.AsBorrow(); b.Flags()&syntax.FlagMut != 0 && s.field == "" && s.ownsFieldPlace(t, b.Operand().ID()) {
			return true
		}
	}
	for ch := range v.Children() {
		if s.scan(t, ch.ID()) {
			return true
		}
	}
	return false
}

func (s *selfScan) selfMember(name string) bool {
	if s.owner == 0 {
		return true
	}
	d := s.c.r.typeDecl(s.owner)
	if f := s.c.r.findField(s.owner, name); f != 0 {
		return name == s.field
	}
	set := d.Members[name]
	if set == 0 {
		return true
	}
	for _, mm := range s.c.r.Overloads[set].Members {
		switch s.c.r.Fn(mm).Recv {
		case RecvSelf:
		case RecvMut:
			if s.mayReplace(mm) {
				return true
			}
		default:
			return true
		}
	}
	return false
}

func rootedAtSelf(t *syntax.Tree, n syntax.NodeID, root string, indexed bool) bool {
	switch t.Kind(n) {
	case syntax.Ident:
		return indexed && t.View(n).Tok().Text() == root
	case syntax.Paren:
		return rootedAtSelf(t, t.View(n).AsParen().Inner().ID(), root, indexed)
	case syntax.MemberExpr:
		return rootedAtSelf(t, t.View(n).AsMember().Base().ID(), root, indexed)
	case syntax.BracketExpr:
		return rootedAtSelf(t, t.View(n).AsBracket().Base().ID(), root, true)
	}
	return false
}
