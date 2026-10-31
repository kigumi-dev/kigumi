package syntax

import "kigumi/internal/lang/token"

type thunkParam struct{ tok, typ uint32 }

// addDefaultThunks hangs a hidden `fn name(self, earlier params) -> T { default }` on each defaulted parameter, without `mut self`, whose cell is no plain `self`.
func (p *parser) addDefaultThunks(mods, name uint32, recv, generics, params NodeID) {
	list := p.tree.nodes[params]
	var earlier []thunkParam
	var self uint32
	for _, id := range p.tree.extra[list.Lhs:list.Rhs] {
		n := p.tree.nodes[id]
		rec := p.tree.extra[n.Lhs:n.Rhs]
		flags, typ, def := rec[1], rec[2], NodeID(rec[3])
		if flags&FlagSelf != 0 {
			if flags&FlagMut == 0 {
				self = n.Tok + 1
			}
			continue
		}
		if flags&FlagVariadic != 0 || typ == 0 {
			continue
		}
		if def != 0 {
			p.tree.extra[n.Lhs+4] = uint32(p.defaultThunk(mods, name, recv, generics, self, earlier, typ, def))
		}
		earlier = append(earlier, thunkParam{n.Tok, typ})
	}
}

// self is the receiver parameter's token index plus one; 0 for none.
func (p *parser) defaultThunk(mods, name uint32, recv, generics NodeID, self uint32, earlier []thunkParam, typ uint32, def NodeID) NodeID {
	tok := p.tree.nodes[def].Tok
	var ps []NodeID
	if self != 0 {
		ps = append(ps, p.tree.addRec(Param, self-1, []uint32{0, FlagSelf, 0, 0, 0}))
	} else if recv != 0 && p.tree.nodes[recv].Rhs == 0 {
		recv = 0
	}
	for _, e := range earlier {
		ps = append(ps, p.tree.addRec(Param, e.tok, []uint32{0, 0, p.thunkType(generics, e.typ), 0, 0}))
	}
	body := p.tree.addList(Block, tok, []NodeID{p.node1(ExprStmt, tok, def)})
	ret := NodeID(p.thunkType(generics, typ))
	keep := mods &^ (ModAsync | ModPureVar | ModStrayBang)
	if p.tree.hasOwnTry(def) {
		ret = p.node1(TypeResult, tok, ret)
		keep |= ModDefaultTry
	}
	return p.tree.addRec(FnDecl, name, []uint32{0, 0, keep, 0, uint32(p.tree.clone(recv)), name, uint32(p.tree.clone(generics)), uint32(p.tree.addList(List, tok, ps)), uint32(ret), uint32(body)})
}

// IsDefaultThunk reports a hidden default thunk: a FnDecl whose head token is its name, not `fn`.
func (t *Tree) IsDefaultThunk(id NodeID) bool {
	n := t.nodes[id]
	return n.Kind == FnDecl && t.Toks[n.Tok].Kind != token.KwFn
}

// hasOwnTry reports a `?` in the expression that belongs to the expression itself, not to a lambda inside it.
func (t *Tree) hasOwnTry(id NodeID) bool {
	switch t.nodes[id].Kind {
	case TryExpr:
		return true
	case Lambda:
		return false
	}
	for c := range t.View(id).Children() {
		if t.hasOwnTry(c.ID()) {
			return true
		}
	}
	return false
}

// thunkType is typ itself, or a copy when the thunk declares its own generics, so each declaration resolves its own type parameters.
func (p *parser) thunkType(generics NodeID, typ uint32) uint32 {
	if generics == 0 {
		return typ
	}
	return uint32(p.tree.clone(NodeID(typ)))
}
