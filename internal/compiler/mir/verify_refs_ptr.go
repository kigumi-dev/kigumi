package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
)

const (
	ptrChunkSize = 64
	ptrMidSize   = 64
	ptrMidSpan   = ptrChunkSize * ptrMidSize
)

type ptrChunk struct {
	owner int32
	v     [ptrChunkSize]int32
}

type ptrMid struct {
	owner  int32
	chunks [ptrMidSize]*ptrChunk
}

// ptrVec maps each local to its object set. Nodes are shared with clones and copied by the first state that writes them; a node belongs to the state whose id it carries.
type ptrVec struct {
	top  []*ptrMid
	id   int32
	sets *objSets
}

func newPtrVec(n int, sets *objSets, fill int32) ptrVec {
	chunk := &ptrChunk{}
	for i := range chunk.v {
		chunk.v[i] = fill
	}
	mid := &ptrMid{}
	for i := range mid.chunks {
		mid.chunks[i] = chunk
	}
	v := ptrVec{top: make([]*ptrMid, (n+ptrMidSpan-1)/ptrMidSpan), id: sets.owner(), sets: sets}
	for i := range v.top {
		v.top[i] = mid
	}
	return v
}

func (v *ptrVec) get(l midir.LocalID) int32 {
	return v.top[l/ptrMidSpan].chunks[l/ptrChunkSize%ptrMidSize].v[l%ptrChunkSize]
}

func (v *ptrVec) set(l midir.LocalID, p int32) {
	if v.get(l) == p {
		return
	}
	t, c := l/ptrMidSpan, l/ptrChunkSize%ptrMidSize
	if v.top[t].owner != v.id {
		cp := *v.top[t]
		cp.owner = v.id
		v.top[t] = &cp
	}
	mid := v.top[t]
	if mid.chunks[c].owner != v.id {
		cp := *mid.chunks[c]
		cp.owner = v.id
		mid.chunks[c] = &cp
	}
	mid.chunks[c].v[l%ptrChunkSize] = p
	v.sets.note(l, p)
}

func (v *ptrVec) clone() ptrVec {
	v.id = v.sets.owner()
	return ptrVec{top: slices.Clone(v.top), id: v.sets.owner(), sets: v.sets}
}

func (v *ptrVec) join(o *ptrVec) bool {
	changed := false
	for t := range v.top {
		if v.top[t] == o.top[t] {
			continue
		}
		for c := range v.top[t].chunks {
			if v.top[t].chunks[c] == o.top[t].chunks[c] {
				continue
			}
			for i, p := range o.top[t].chunks[c].v {
				l := midir.LocalID(t*ptrMidSpan + c*ptrChunkSize + i)
				if cur := v.get(l); cur != p {
					if u := v.sets.union(cur, p); u != cur {
						v.set(l, u)
						changed = true
					}
				}
			}
		}
	}
	return changed
}
