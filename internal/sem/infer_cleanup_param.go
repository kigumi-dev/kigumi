package sem

import "kigumi/internal/syntax"

func (c *checker) mutBorrowKeeps(at syntax.NodeID, r reservation, borrowed []EntityID) bool {
	param := c.held.mutArgParam[at]
	if param == 0 {
		return false
	}
	s := selfScan{c: c, field: c.reservedBelow(r, borrowed), seen: map[selfScanKey]bool{}}
	return !s.mayReplaceParam(param)
}

func (c *checker) noteMutArg(a syntax.NodeID, param EntityID) {
	if param == 0 || c.t.Kind(a) != syntax.BorrowExpr || c.t.View(a).AsBorrow().Flags()&syntax.FlagMut == 0 {
		return
	}
	if c.held.mutArgParam == nil {
		c.held.mutArgParam = map[syntax.NodeID]EntityID{}
	}
	c.held.mutArgParam[a] = param
}

// reserveDrainArg runs after the argument is synthesized: reserving a field place reads its type.
func (c *checker) reserveDrainArg(a syntax.NodeID, param EntityID) {
	if c.cleanup > 0 && c.held.mutArgParam[a] == param && param != 0 && c.paramDrains(param, map[EntityID]bool{}) {
		c.reserveUse(c.t.View(a).AsBorrow().Operand().ID())
	}
}

func (s *selfScan) mayReplaceParam(param EntityID) bool {
	pe := &s.c.r.Entities[param]
	fn := pe.Parent
	if pe.Kind != EntParam || s.c.r.Entities[fn].Kind != EntFn {
		return true
	}
	info := s.c.r.Fn(fn)
	if info.Body == 0 {
		return true
	}
	key := selfScanKey{param, s.field}
	if s.seen[key] {
		return false
	}
	s.seen[key] = true
	tt := s.c.r.Types
	if tt.Kind(pe.Type) != KRef {
		return true
	}
	if tt.Node(pe.Type).Flags&flagMut == 0 {
		return false
	}
	sub := *s
	sub.root, sub.fn = pe.Name, fn
	sub.owner = sub.typeEnt(tt.Node(pe.Type).Elem)
	return sub.scan(s.c.r.tree(pe.File), info.Body)
}

// scanCall follows a root passed on to a plain function; any other use of the bare root stays a replacement.
func (s *selfScan) scanCall(t *syntax.Tree, v syntax.View) (handled, replaced bool) {
	call := v.AsCall()
	callee := t.Unparen(call.Callee().ID())
	if t.Kind(callee) != syntax.Ident || t.View(callee).Tok().Text() == s.root {
		return false, false
	}
	fn := s.pkgFn(t, t.View(callee).Tok().Text())
	if fn == 0 {
		return false, false
	}
	params := s.c.r.Fn(fn).Params
	i := 0
	for a := range call.Args().IDs() {
		arg := t.Unparen(a)
		if t.Kind(arg) == syntax.Ident && t.View(arg).Tok().Text() == s.root {
			if i >= len(params) || s.mayReplaceParam(params[i]) {
				return true, true
			}
		} else if s.scan(t, arg) {
			return true, true
		}
		i++
	}
	return true, false
}

// pkgFn resolves a callee name in the scanned function's package; a parameter or local of that name shadows it and resolves to nothing.
func (s *selfScan) pkgFn(t *syntax.Tree, name string) EntityID {
	if s.fn == 0 || s.shadowed(t, name) {
		return 0
	}
	r := s.c.r
	b := r.Scopes[r.Packages[r.Entities[s.fn].Pkg].Scope].Names[name]
	if b.Set == 0 || len(r.Overloads[b.Set].Members) != 1 {
		return 0
	}
	if m := r.Overloads[b.Set].Members[0]; r.Entities[m].Kind == EntFn && r.Fn(m).Owner == 0 {
		return m
	}
	return 0
}

func (s *selfScan) shadowed(t *syntax.Tree, name string) bool {
	info := s.c.r.Fn(s.fn)
	for _, p := range info.Params {
		if s.c.r.Entities[p].Name == name {
			return true
		}
	}
	return bindsName(t, info.Body, name)
}

func bindsName(t *syntax.Tree, n syntax.NodeID, name string) bool {
	if k := t.Kind(n); (k == syntax.PatBind || k == syntax.Param) && t.View(n).Tok().Text() == name {
		return true
	}
	for ch := range t.View(n).Children() {
		if bindsName(t, ch.ID(), name) {
			return true
		}
	}
	return false
}
