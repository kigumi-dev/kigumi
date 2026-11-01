package build

import (
	"kigumi/internal/compiler/hir"
	"kigumi/internal/compiler/sem"
)

func patBinds(p *hir.Pat, into map[sem.EntityID]bool) {
	if p == nil {
		return
	}
	if p.Kind == hir.PatBind {
		into[p.Ent] = true
	}
	for _, s := range p.Subs {
		patBinds(s, into)
	}
	for _, f := range p.Fields {
		if f.Bind != 0 {
			into[f.Bind] = true
		}
		patBinds(f.Sub, into)
	}
}

// mentions reports whether e reads or captures one of ents anywhere outside skip.
func mentions(e, skip *hir.Expr, ents map[sem.EntityID]bool) bool {
	if e == nil || e == skip {
		return false
	}
	if e.Kind == hir.Local && ents[e.Ent] {
		return true
	}
	if e.Fn != nil {
		for _, c := range e.Fn.Captures {
			if ents[c.Ent] {
				return true
			}
		}
	}
	kids := append([]*hir.Expr{e.Cond, e.Else, e.Guard}, e.Args...)
	kids = append(kids, e.CtxArgs...)
	for _, en := range e.Entries {
		kids = append(kids, en.Value)
	}
	for _, arm := range e.Arms {
		kids = append(kids, arm.Guard, arm.Body)
	}
	for _, k := range kids {
		if mentions(k, skip, ents) {
			return true
		}
	}
	return blockMentions(e.Then, skip, ents) || blockMentions(e.Block, skip, ents)
}

func blockMentions(blk *hir.Block, skip *hir.Expr, ents map[sem.EntityID]bool) bool {
	if blk == nil {
		return false
	}
	for _, s := range blk.Stmts {
		if stmtMentions(s, skip, ents) {
			return true
		}
	}
	return mentions(blk.Tail, skip, ents)
}

func stmtMentions(s *hir.Stmt, skip *hir.Expr, ents map[sem.EntityID]bool) bool {
	if s == nil {
		return false
	}
	if pl := s.Place; pl != nil && (ents[pl.Ent] || mentions(pl.Expr, skip, ents) || mentions(pl.Base, skip, ents) || mentions(pl.Idx, skip, ents)) {
		return true
	}
	return mentions(s.Expr, skip, ents) || blockMentions(s.Else, skip, ents) || stmtMentions(s.Body, skip, ents) || blockMentions(s.Block, skip, ents)
}
