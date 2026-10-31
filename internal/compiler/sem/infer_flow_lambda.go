package sem

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// lambdaCalls collects, per lambda bound by `let`, where its binding is called or passed to a call.
type lambdaCalls struct {
	r      *Result
	file   FileID
	tree   *syntax.Tree
	scan   *flowScan
	bound  map[EntityID]token.Span
	direct map[syntax.NodeID]bool
	bad    map[token.Span]bool
}

func (lc *lambdaCalls) visit(n syntax.NodeID) {
	t := lc.tree
	switch t.Kind(n) {
	case syntax.LetStmt:
		s := letStmt(t, n)
		if s.Init != 0 && t.Kind(s.Init) == syntax.Lambda && s.Pattern != 0 && t.Kind(s.Pattern) == syntax.PatBind {
			if local := lc.r.Files[lc.file].Defs[s.Pattern]; local != 0 {
				lc.bound[local] = t.Span(s.Init)
				if lc.scan.calls == nil {
					lc.scan.calls = map[token.Span][]token.Span{}
				}
				lc.scan.calls[t.Span(s.Init)] = []token.Span{}
			}
		}
	case syntax.CallExpr, syntax.WsCallExpr:
		call := t.View(n).AsCallLike()
		lc.direct[call.Callee().ID()] = true
		for a := range call.Args().IDs() {
			lc.direct[a] = true
		}
	case syntax.Ident:
		lam, ok := lc.bound[lc.r.Files[lc.file].Uses[n]]
		if !ok {
			return
		}
		if lc.direct[n] {
			lc.scan.calls[lam] = append(lc.scan.calls[lam], t.Span(n))
		} else {
			lc.bad[lam] = true
		}
	}
}

func (lc *lambdaCalls) finish() {
	for lam := range lc.bad {
		delete(lc.scan.calls, lam)
	}
}

// runSpans are where a write at site runs: the call sites of a let-bound lambda holding it, else site itself.
func (s *flowScan) runSpans(site, cut token.Span) []token.Span {
	var lam token.Span
	for _, l := range s.lambdas {
		if within(l, site) && !within(l, cut) && (lam == token.Span{} || within(lam, l) && l != lam) {
			lam = l
		}
	}
	uses, ok := s.calls[lam]
	if !ok || len(uses) == 0 {
		return []token.Span{site}
	}
	for _, u := range uses {
		for _, l := range s.lambdas {
			if within(l, u) {
				return []token.Span{site}
			}
		}
	}
	return uses
}
