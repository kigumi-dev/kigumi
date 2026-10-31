package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// A value with no entry in a facts map may hold any variant.
type variantSet struct {
	list []sem.EntityID
	all  int
}

func (a variantSet) full() bool { return len(a.list) == a.all }

func meet(a, b variantSet) variantSet {
	out := variantSet{all: a.all}
	for _, v := range a.list {
		if slices.Contains(b.list, v) {
			out.list = append(out.list, v)
		}
	}
	return out
}

func joinSets(a, b variantSet) variantSet {
	list := append(slices.Clone(a.list), b.list...)
	slices.Sort(list)
	return variantSet{list: slices.Compact(list), all: a.all}
}

func (a variantSet) equal(b variantSet) bool { return slices.Equal(a.list, b.list) }

type vplace struct {
	root, rel midir.LocalID
	path      uint32
	tag       uint8
}

const (
	tagValue uint8 = iota
	tagAssigned
	tagEmptied
	tagBelow
)

func (pl vplace) about(l midir.LocalID) bool {
	return pl.root == l || (pl.tag == tagBelow && pl.rel == l)
}

type pathIDs struct {
	ids map[[2]uint32]uint32
}

func (p *pathIDs) child(parent uint32, slot int) uint32 {
	key := [2]uint32{parent, uint32(slot)}
	if id, ok := p.ids[key]; ok {
		return id
	}
	id := uint32(len(p.ids)) + 1
	p.ids[key] = id
	return id
}
