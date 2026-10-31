package lsp

import (
	"sort"

	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

type spanNode struct {
	sp   token.Span
	node syntax.NodeID
}

// posIndex finds the name-bearing node at a byte offset without walking the
// whole tree; spans come from the tree's span cache.
type posIndex struct {
	names []spanNode
	// maxEnd[i] is the furthest end among names[:i+1], which bounds how far back a lookup scans.
	maxEnd []token.Pos
}

func (snap *snapshot) posIndexOf(t *syntax.Tree) *posIndex {
	snap.posMu.Lock()
	defer snap.posMu.Unlock()
	if pi, ok := snap.pos[t]; ok {
		return pi
	}
	pi := newPosIndex(t)
	snap.pos[t] = pi
	return pi
}

func isNameNode(k syntax.NodeKind) bool {
	switch k {
	case syntax.Ident, syntax.MemberExpr, syntax.PatBind, syntax.Param, syntax.PatCtor, syntax.TypePath, syntax.FieldInit, syntax.PatField:
		return true
	}
	return false
}

func newPosIndex(t *syntax.Tree) *posIndex {
	pi := &posIndex{}
	for v := range t.All() {
		if !isNameNode(v.Kind()) {
			continue
		}
		sp := v.Span()
		if v.Kind() == syntax.MemberExpr {
			sp = v.Tok().Span()
		}
		pi.names = append(pi.names, spanNode{sp, v.ID()})
	}
	sort.SliceStable(pi.names, func(i, j int) bool { return pi.names[i].sp.Start < pi.names[j].sp.Start })
	pi.maxEnd = make([]token.Pos, len(pi.names))
	var m token.Pos
	for i, n := range pi.names {
		m = max(m, n.sp.End)
		pi.maxEnd[i] = m
	}
	return pi
}

// nodeAt is the innermost name-bearing node covering pos; the lowest node id wins a tie.
func (pi *posIndex) nodeAt(pos token.Pos) syntax.NodeID {
	hi := sort.Search(len(pi.names), func(i int) bool { return pi.names[i].sp.Start > pos })
	best, bestLen := syntax.NodeID(0), -1
	for i := hi - 1; i >= 0 && pi.maxEnd[i] >= pos; i-- {
		n := pi.names[i]
		if pos > n.sp.End {
			continue
		}
		l := n.sp.Len()
		if bestLen < 0 || l < bestLen || l == bestLen && n.node < best {
			best, bestLen = n.node, l
		}
	}
	return best
}
