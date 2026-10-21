package mir

import "kigumi/internal/syntax"

// ownVec holds ownership states by local; an unset entry reads as ownUninit but joins differently from a stored one.
type ownVec struct{ v cowVec[uint8] }

func (o *ownVec) get(l LocalID) ownState {
	if x := o.v.get(l); x != 0 {
		return ownState(x - 1)
	}
	return ownUninit
}

func (o *ownVec) set(l LocalID, s ownState) { o.v.set(l, uint8(s)+1) }

func (o *ownVec) del(l LocalID) { o.v.set(l, 0) }

func (o *ownVec) clone() ownVec { return ownVec{o.v.clone()} }

func (o *ownVec) join(other *ownVec) bool {
	return o.v.join(&other.v, func(cur, x uint8) uint8 {
		switch {
		case x == 0:
			return cur
		case cur == 0:
			return x
		}
		return uint8(joinOwn(ownState(cur-1), ownState(x-1))) + 1
	})
}

// nodeVec holds the node of an event by local; zero means none.
type nodeVec struct{ v cowVec[syntax.NodeID] }

func (n *nodeVec) get(l LocalID) syntax.NodeID { return n.v.get(l) }

func (n *nodeVec) set(l LocalID, id syntax.NodeID) { n.v.set(l, id) }

func (n *nodeVec) del(l LocalID) { n.v.set(l, 0) }

func (n *nodeVec) clone() nodeVec { return nodeVec{n.v.clone()} }

func (n *nodeVec) join(other *nodeVec) {
	n.v.join(&other.v, func(cur, x syntax.NodeID) syntax.NodeID {
		if cur != 0 {
			return cur
		}
		return x
	})
}
