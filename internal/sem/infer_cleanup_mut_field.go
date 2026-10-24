package sem

import "kigumi/internal/syntax"

func (s *selfScan) selfFieldType(t *syntax.Tree, n syntax.NodeID) (TypeID, int, bool) {
	switch t.Kind(n) {
	case syntax.Ident:
		return 0, 0, t.View(n).Tok().Text() == s.root
	case syntax.Paren:
		return s.selfFieldType(t, t.View(n).AsParen().Inner().ID())
	case syntax.MemberExpr:
		v := t.View(n)
		bt, depth, ok := s.selfFieldType(t, v.AsMember().Base().ID())
		if !ok {
			return 0, 0, false
		}
		owner := s.owner
		if depth > 0 {
			owner = s.typeEnt(bt)
		}
		if owner == 0 {
			return 0, 0, false
		}
		f := s.c.r.findField(owner, v.Tok().Text())
		if f == 0 {
			return 0, 0, false
		}
		return s.c.r.Field(f).Type, depth + 1, true
	}
	return 0, 0, false
}

func (s *selfScan) typeEnt(ty TypeID) EntityID {
	ent := s.c.r.Types.Node(ty).Ent
	if ent == 0 || s.c.r.Entities[ent].Kind != EntType {
		return 0
	}
	return ent
}

func (s *selfScan) ownsFieldPlace(t *syntax.Tree, n syntax.NodeID) bool {
	ty, depth, ok := s.selfFieldType(t, n)
	return ok && depth > 0 && !s.c.r.IsCopy(ty)
}

func (s *selfScan) fieldMethodMayReplace(t *syntax.Tree, v syntax.View) bool {
	base := v.AsMember().Base().ID()
	ty, depth, ok := s.selfFieldType(t, base)
	if !ok || depth == 0 || s.c.r.IsCopy(ty) {
		return false
	}
	name := v.Tok().Text()
	owner := s.typeEnt(ty)
	if owner == 0 {
		return true
	}
	if s.c.r.findField(owner, name) != 0 {
		return false
	}
	set := s.c.r.typeDecl(owner).Members[name]
	if set == 0 {
		return true
	}
	for _, mm := range s.c.r.Overloads[set].Members {
		switch s.c.r.Fn(mm).Recv {
		case RecvSelf:
		case RecvMut:
			sub := selfScan{c: s.c, owner: s.c.r.Fn(mm).Owner, root: "self", seen: s.seen}
			if sub.mayReplace(mm) {
				return true
			}
		default:
			return true
		}
	}
	return false
}

// firstSelfField names the field of self a field path n starts with, "" when n is not one.
func (s *selfScan) firstSelfField(t *syntax.Tree, n syntax.NodeID) string {
	for {
		switch t.Kind(n) {
		case syntax.Paren:
			n = t.View(n).AsParen().Inner().ID()
			continue
		case syntax.MemberExpr:
			base := t.Unparen(t.View(n).AsMember().Base().ID())
			if t.Kind(base) == syntax.Ident && t.View(base).Tok().Text() == s.root {
				return t.View(n).Tok().Text()
			}
			n = base
			continue
		}
		return ""
	}
}

// reservedPartWrite reports whether writing or lending n mutably may replace the reserved field or a resource inside it.
func (s *selfScan) reservedPartWrite(t *syntax.Tree, n syntax.NodeID) bool {
	if s.field == "" || s.firstSelfField(t, n) != s.field {
		return false
	}
	ty, depth, ok := s.selfFieldType(t, n)
	return !ok || depth <= 1 || s.c.r.mayHoldResource(ty, map[TypeID]bool{})
}

// reservedPartUse judges a member of a path inside the reserved field: reading a field keeps it, a method keeps it unless it may replace its receiver.
func (s *selfScan) reservedPartUse(t *syntax.Tree, v syntax.View) (bool, bool) {
	base := v.AsMember().Base().ID()
	if s.field == "" || s.firstSelfField(t, base) != s.field {
		return false, false
	}
	ty, _, ok := s.selfFieldType(t, base)
	if !ok {
		return true, true
	}
	if owner := s.typeEnt(ty); owner != 0 && s.c.r.findField(owner, v.Tok().Text()) != 0 {
		return true, false
	}
	return true, s.fieldMethodMayReplace(t, v)
}
