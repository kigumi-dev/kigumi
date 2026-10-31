package syntax

import (
	"kigumi/internal/lang/diag"
	"kigumi/internal/lang/token"
)

// NodeID indexes Tree.nodes; 0 is the absent node.
type NodeID uint32

type rawNode struct {
	Kind NodeKind
	Tok  uint32
	Lhs  uint32
	Rhs  uint32
}

// Tree is the flat, lossless syntax tree; nodes reference each other and the token array by index only, with semantics kept in side tables.
type Tree struct {
	File  *token.File
	Toks  []token.Token
	nodes []rawNode
	extra []uint32
	Root  NodeID
	Diags []diag.Diagnostic
	// DiagsDropped is how many further diagnostics the bag's limit cut off.
	DiagsDropped int
	// interp is the parser's interpolation-range cache (see parser.interp),
	// kept so StringParts reuses it instead of rescanning nested strings.
	interp map[int]interpInfo
	// spanCache memoizes Span per node id, since every hir/sem/mir lookup calls it and an uncached chain of D nested nodes would cost O(D²) total.
	spanCache []token.Span
	spanKnown []bool
}

func newTree(f *token.File, toks []token.Token) *Tree {
	return &Tree{File: f, Toks: toks, nodes: make([]rawNode, 1, 64)}
}

func (t *Tree) add(n rawNode) NodeID {
	t.nodes = append(t.nodes, n)
	return NodeID(len(t.nodes) - 1)
}

// AddList appends a list node; the checker uses it to group nodes.
func (t *Tree) AddList(kind NodeKind, tok uint32, items []NodeID) NodeID {
	return t.addList(kind, tok, items)
}

// AddDefaultArg appends the stand-in for an omitted defaulted argument of the call whose head token is tok.
func (t *Tree) AddDefaultArg(tok uint32) NodeID {
	return t.add(rawNode{Kind: DefaultArg, Tok: tok})
}

func (t *Tree) addList(kind NodeKind, tok uint32, items []NodeID) NodeID {
	start := uint32(len(t.extra))
	for _, it := range items {
		t.extra = append(t.extra, uint32(it))
	}
	return t.add(rawNode{Kind: kind, Tok: tok, Lhs: start, Rhs: uint32(len(t.extra))})
}

func (t *Tree) addRec(kind NodeKind, tok uint32, slots []uint32) NodeID {
	start := uint32(len(t.extra))
	t.extra = append(t.extra, slots...)
	return t.add(rawNode{Kind: kind, Tok: tok, Lhs: start, Rhs: uint32(len(t.extra))})
}

func (p *parser) leaf(kind NodeKind, tok uint32) NodeID {
	return p.tree.add(rawNode{Kind: kind, Tok: tok})
}

func (p *parser) node1(kind NodeKind, tok uint32, lhs NodeID) NodeID {
	return p.tree.add(rawNode{Kind: kind, Tok: tok, Lhs: uint32(lhs)})
}

func (p *parser) node2(kind NodeKind, tok uint32, lhs, rhs NodeID) NodeID {
	return p.tree.add(rawNode{Kind: kind, Tok: tok, Lhs: uint32(lhs), Rhs: uint32(rhs)})
}

func (p *parser) nodeFlag(kind NodeKind, tok uint32, flag uint32, rhs NodeID) NodeID {
	return p.tree.add(rawNode{Kind: kind, Tok: tok, Lhs: flag, Rhs: uint32(rhs)})
}

func (t *Tree) Kind(id NodeID) NodeKind {
	if id == 0 {
		return NoneKind
	}
	return t.nodes[id].Kind
}

func (t *Tree) TokText(idx uint32) string {
	return t.Toks[idx].Text(t.File.Src)
}

// DropRootItems removes the first n items of the root list, so a script's
// leading manifest records never reach the checker.
func (t *Tree) DropRootItems(n int) {
	t.nodes[t.Root].Lhs += uint32(n)
	if int(t.Root) < len(t.spanKnown) {
		t.spanKnown[t.Root] = false
	}
}
