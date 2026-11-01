package lsp

import (
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// The scans below are the lookups the indexes replaced; the index tests
// require identical answers.

func scanNodeAt(t *syntax.Tree, pos token.Pos) syntax.NodeID {
	best, bestLen := syntax.NodeID(0), -1
	for v := range t.All() {
		if !isNameNode(v.Kind()) {
			continue
		}
		sp := v.Span()
		if v.Kind() == syntax.MemberExpr {
			sp = v.Tok().Span()
		}
		if pos >= sp.Start && pos <= sp.End && (bestLen < 0 || sp.Len() < bestLen) {
			best, bestLen = v.ID(), sp.Len()
		}
	}
	return best
}

func scanDeclaredAt(res *sem.Result, t *syntax.Tree, pos token.Pos) sem.EntityID {
	for i := range res.Entities {
		e := &res.Entities[i]
		if e.File == 0 || e.Tok == 0 || res.Tree(e.File) != t {
			continue
		}
		if sp := t.Toks[e.Tok].Span; pos >= sp.Start && pos <= sp.End {
			return sem.EntityID(i)
		}
	}
	return 0
}

func scanUses(res *sem.Result, targets map[sem.EntityID]bool) []useSite {
	var out []useSite
	for i := 1; i < len(res.Files); i++ {
		f := &res.Files[i]
		if f.Tree.File.Generated {
			continue
		}
		seen := map[token.Span]bool{}
		for n, use := range f.Uses {
			if !targets[use] {
				continue
			}
			if sp := nameSpan(f.Tree, syntax.NodeID(n)); !seen[sp] {
				seen[sp] = true
				out = append(out, useSite{i, f.Tree, syntax.NodeID(n)})
			}
		}
	}
	return out
}

func scanImplementations(res *sem.Result, ent sem.EntityID) []sem.EntityID {
	e := res.Entity(ent)
	var out []sem.EntityID
	switch {
	case e.Kind == sem.EntInterface:
		for _, ty := range declared(res, sem.EntType) {
			if _, ok := conforms(res, ty, ent); ok {
				out = append(out, ty)
			}
		}
	case e.Kind == sem.EntType:
		for _, iface := range declared(res, sem.EntInterface) {
			if _, ok := conforms(res, ent, iface); ok {
				out = append(out, iface)
			}
		}
	case isMethod(res, ent) && res.Entity(res.Fn(ent).Owner).Kind == sem.EntInterface:
		iface := res.Fn(ent).Owner
		for _, ty := range declared(res, sem.EntType) {
			if witnesses, ok := conforms(res, ty, iface); ok {
				for i, req := range res.Iface(iface).Reqs {
					if req == ent {
						out = append(out, witnesses[i])
					}
				}
			}
		}
	case isMethod(res, ent):
		owner := res.Fn(ent).Owner
		for _, iface := range declared(res, sem.EntInterface) {
			if witnesses, ok := conforms(res, owner, iface); ok {
				for i, w := range witnesses {
					if w == ent {
						out = append(out, res.Iface(iface).Reqs[i])
					}
				}
			}
		}
	}
	return out
}
