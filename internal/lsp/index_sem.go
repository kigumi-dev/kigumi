package lsp

import (
	"slices"
	"sort"

	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

type useSite struct {
	file int
	tree *syntax.Tree
	node syntax.NodeID
}

type declTok struct {
	sp  token.Span
	ent sem.EntityID
}

// semIndex answers definition and reference lookups from tables built once
// per snapshot instead of scanning every file or entity per request.
type semIndex struct {
	uses  map[sem.EntityID][]useSite
	decls map[*syntax.Tree][]declTok
	// imports lists the selected-import bindings that forward to an entity.
	imports map[sem.EntityID][]sem.EntityID
}

func (snap *snapshot) semIndex() *semIndex {
	snap.idxOnce.Do(func() { snap.idx = buildSemIndex(snap.res) })
	return snap.idx
}

func buildSemIndex(res *sem.Result) *semIndex {
	ix := &semIndex{uses: map[sem.EntityID][]useSite{}, decls: map[*syntax.Tree][]declTok{}, imports: map[sem.EntityID][]sem.EntityID{}}
	for i := 1; i < len(res.Files); i++ {
		f := &res.Files[i]
		if f.Tree.File.Generated {
			continue
		}
		for n, use := range f.Uses {
			if use != 0 {
				ix.uses[use] = append(ix.uses[use], useSite{i, f.Tree, syntax.NodeID(n)})
			}
		}
	}
	for i := range res.Entities {
		e := &res.Entities[i]
		if e.File == 0 || e.Tok == 0 {
			continue
		}
		t := res.Tree(e.File)
		if e.Kind == sem.EntImport && e.Target != 0 && !t.File.Generated {
			ix.imports[e.Target] = append(ix.imports[e.Target], sem.EntityID(i))
		}
		ix.decls[t] = append(ix.decls[t], declTok{t.Toks[e.Tok].Span, sem.EntityID(i)})
	}
	for _, list := range ix.decls {
		sort.SliceStable(list, func(i, j int) bool { return list[i].sp.Start < list[j].sp.Start })
	}
	return ix
}

// declaredAt finds the entity whose declaring name token covers pos; the
// lowest entity id wins when tokens touch or are shared.
func (ix *semIndex) declaredAt(t *syntax.Tree, pos token.Pos) sem.EntityID {
	list := ix.decls[t]
	hi := sort.Search(len(list), func(i int) bool { return list[i].sp.Start > pos })
	var best sem.EntityID
	for i := hi - 1; i >= 0 && pos-list[i].sp.Start <= maxNameLen; i-- {
		if list[i].sp.End >= pos && (best == 0 || list[i].ent < best) {
			best = list[i].ent
		}
	}
	return best
}

// maxNameLen bounds how far back declaredAt scans: a name token is never longer.
const maxNameLen = 1 << 12

// usesOf lists the use sites of every target in file then node order.
func (ix *semIndex) usesOf(targets []sem.EntityID) []useSite {
	var out []useSite
	for _, id := range targets {
		out = append(out, ix.uses[id]...)
	}
	slices.SortStableFunc(out, func(a, b useSite) int {
		if a.file != b.file {
			return a.file - b.file
		}
		return int(a.node) - int(b.node)
	})
	// A type annotation records both its Path and its TypePath node on the same name.
	type key struct {
		tree *syntax.Tree
		sp   token.Span
	}
	seen := map[key]bool{}
	return slices.DeleteFunc(out, func(u useSite) bool {
		k := key{u.tree, nameSpan(u.tree, u.node)}
		dup := seen[k]
		seen[k] = true
		return dup
	})
}
