package sem

import "kigumi/internal/syntax"

func (c *checker) lambdaParams(closure EntityID, params []syntax.NodeID, target TypeID) ([]TypeID, []EntityID, bool) {
	tt := c.r.Types
	var types []TypeID
	var ents []EntityID
	bad := false
	for i, p := range params {
		ps := param(c.t, p)
		name := c.t.View(p).Tok().Text()
		var pt TypeID
		declared := false
		if target != 0 {
			pt = c.vars.settleLiterals(tt.Node(target).Args[i])
		}
		switch {
		case ps.Type != 0:
			annotated := c.resolveLocalType(ps.Type)
			if ps.Flags&syntax.FlagEscaping != 0 {
				annotated, declared = c.r.escapingParamType(c.f, p, annotated)
			}
			if pt != 0 && annotated != pt && !c.vars.unify(annotated, pt) {
				c.errAt(p, cLambdaParamMismatch, name, annotated, pt)
				bad = true
			}
			pt = annotated
		case pt == 0 || !c.vars.onlyLifetimeVars(pt):
			c.errAt(p, cLambdaParamAnnot, name)
			pt = TyPoison
			bad = true
		}
		pe := Entity{Kind: EntParam, Name: name, Pkg: c.pkg, File: c.f, Node: p, Tok: c.t.View(p).Tok().Index(), Parent: closure, Type: pt}
		if ps.Flags&syntax.FlagMut != 0 {
			pe.Flags |= EfMut
		}
		if declared || tt.FnIsEscaping(pt) || target != 0 && tt.FnParamEscapes(target, i) {
			pe.Flags |= EfEscaping
		}
		pid := c.r.newEntity(pe)
		c.r.Entities[pid].Detail = c.r.addLocal(LocalInfo{Scope: c.scope})
		c.info.Defs[p] = pid
		if _, dup := c.r.Scopes[c.scope].Names[name]; dup {
			c.errAt(p, cParamDuplicate, name)
		} else if name != "_" {
			c.r.Scopes[c.scope].Names[name] = Binding{Ent: pid}
		}
		c.setType(p, pt)
		types = append(types, pt)
		ents = append(ents, pid)
	}
	return types, ents, bad
}

func (c *checker) escapingFlags(params []EntityID) []bool {
	out := make([]bool, len(params))
	for i, p := range params {
		out[i] = c.r.Entities[p].Flags&EfEscaping != 0
	}
	return out
}
