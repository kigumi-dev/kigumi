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
			if ps.Flags&syntax.FlagVariadic != 0 {
				if i != len(params)-1 {
					c.errAt(p, cVariadicNotLast)
					bad = true
				}
				annotated = tt.Named(tt.arrayEnt, []TypeID{tt.WithoutEscaping(annotated)})
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
		lt := EntityID(0)
		if tt.Kind(pt) == KRef {
			lt = c.r.explicitLifetimeOf(c.f, c.scope, ps.Type)
		}
		if qualifier := tt.Node(tt.RefLifetime(pt)); lt == 0 && qualifier.Kind == KParam {
			lt = qualifier.Ent
		}
		c.r.Entities[pid].Detail = c.r.addLocal(LocalInfo{Scope: c.scope, Lifetime: lt})
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

// lambdaVariadic reports a lambda whose last parameter is written `name: ...T`, or is bare under an expected variadic fn type.
func (c *checker) lambdaVariadic(params []syntax.NodeID, target TypeID) bool {
	if len(params) == 0 {
		return false
	}
	ps := param(c.t, params[len(params)-1])
	if ps.Flags&syntax.FlagVariadic != 0 {
		return true
	}
	return ps.Type == 0 && target != 0 && c.r.Types.Node(target).Flags&fnVariadic != 0
}
