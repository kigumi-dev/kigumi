package syntax

import "iter"

// ListView is a node whose children are a run of node ids: List, Block, File,
// the type bodies and the other list-layout kinds. On any other node it is empty.
type ListView struct{ View }

func (v View) AsList() ListView         { return ListView{v} }
func (t *Tree) List(id NodeID) ListView { return ListView{View{t, id}} }

func (l ListView) bounds() (lo, hi uint32) {
	n := l.t.nodes[l.id]
	if kindTable[n.Kind].layout != layList {
		return 0, 0
	}
	return n.Lhs, n.Rhs
}

func (l ListView) Len() int {
	lo, hi := l.bounds()
	return int(hi - lo)
}

func (l ListView) ItemID(i int) NodeID {
	lo, hi := l.bounds()
	return NodeID(l.t.extra[lo:hi][i])
}

func (l ListView) At(i int) View { return View{l.t, l.ItemID(i)} }

func (l ListView) First() View {
	if l.Len() == 0 {
		return View{}
	}
	return l.At(0)
}

func (l ListView) Last() View {
	if n := l.Len(); n > 0 {
		return l.At(n - 1)
	}
	return View{}
}

// Cursor iterates a ListView: for c := l.Cursor(); c.Next(); { use(c.View()) }.
type Cursor struct {
	t   *Tree
	ids []uint32
	i   int
	cur NodeID
}

func (l ListView) Cursor() Cursor {
	lo, hi := l.bounds()
	return Cursor{t: l.t, ids: l.t.extra[lo:hi]}
}

func (c *Cursor) Next() bool {
	if c.i >= len(c.ids) {
		return false
	}
	c.cur = NodeID(c.ids[c.i])
	c.i++
	return true
}

func (c *Cursor) ID() NodeID { return c.cur }
func (c *Cursor) View() View { return View{c.t, c.cur} }
func (c *Cursor) Index() int { return c.i - 1 }

// IDs and Views are range-over-func forms of the cursor; neither allocates.
func (l ListView) IDs() iter.Seq[NodeID] {
	return func(yield func(NodeID) bool) {
		for c := l.Cursor(); c.Next(); {
			if !yield(c.cur) {
				return
			}
		}
	}
}

func (l ListView) Views() iter.Seq[View] {
	return func(yield func(View) bool) {
		for c := l.Cursor(); c.Next(); {
			if !yield(View{l.t, c.cur}) {
				return
			}
		}
	}
}

func (l ListView) Enumerate() iter.Seq2[int, View] {
	return func(yield func(int, View) bool) {
		for c := l.Cursor(); c.Next(); {
			if !yield(c.Index(), View{l.t, c.cur}) {
				return
			}
		}
	}
}

// PathView is a Path node: a run of identifier tokens.
type PathView struct{ View }

func (v View) AsPath() PathView { return PathView{v} }

func (p PathView) bounds() (lo, hi uint32) {
	n := p.t.nodes[p.id]
	if n.Kind != Path {
		return 0, 0
	}
	return n.Lhs, n.Rhs
}

func (p PathView) Len() int {
	lo, hi := p.bounds()
	return int(hi - lo)
}

func (p PathView) At(i int) TokenRef {
	lo, _ := p.bounds()
	return TokenRef{p.t, p.t.extra[int(lo)+i]}
}

func (p PathView) Last() TokenRef {
	if n := p.Len(); n > 0 {
		return p.At(n - 1)
	}
	return TokenRef{p.t, 0}
}

func (p PathView) Segments() iter.Seq2[int, TokenRef] {
	return func(yield func(int, TokenRef) bool) {
		for i, n := 0, p.Len(); i < n; i++ {
			if !yield(i, p.At(i)) {
				return
			}
		}
	}
}

// EnumerateIDs is Enumerate over the raw ids.
func (l ListView) EnumerateIDs() iter.Seq2[int, NodeID] {
	return func(yield func(int, NodeID) bool) {
		for c := l.Cursor(); c.Next(); {
			if !yield(c.Index(), c.cur) {
				return
			}
		}
	}
}
