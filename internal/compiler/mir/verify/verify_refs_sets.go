package verify

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
)

type objSets struct {
	sets  [][]int32
	index map[string]int32
	merge map[[2]int32]int32
	one   []int32
	// holders[site] lists every local any state ever gave an object of the site, so a retag visits those rather than all locals.
	holders [][]midir.LocalID
	seen    map[uint64]struct{}
	noted   []int32
	owners  int32
}

func newObjSets(objects, locals int) *objSets {
	return &objSets{one: make([]int32, objects), holders: make([][]midir.LocalID, objects/3+1), seen: map[uint64]struct{}{}, noted: make([]int32, locals)}
}

func (o *objSets) owner() int32 {
	o.owners++
	return o.owners
}

func (o *objSets) note(l midir.LocalID, set int32) {
	if o.noted[l] == set+1 {
		return
	}
	o.noted[l] = set + 1
	for _, x := range o.sets[set] {
		if x < 0 {
			continue
		}
		site := objSite(x)
		key := uint64(site)<<32 | uint64(l)
		if _, ok := o.seen[key]; !ok {
			o.seen[key] = struct{}{}
			o.holders[site] = append(o.holders[site], l)
		}
	}
}

func (o *objSets) single(obj int32) int32 {
	if n := o.one[obj]; n != 0 {
		return n - 1
	}
	n := o.of(obj)
	o.one[obj] = n + 1
	return n
}

func (o *objSets) of(ids ...int32) int32 {
	slices.Sort(ids)
	ids = slices.Compact(ids)
	key := make([]byte, 0, len(ids)*4)
	for _, id := range ids {
		key = append(key, byte(id), byte(id>>8), byte(id>>16), byte(id>>24))
	}
	if n, ok := o.index[string(key)]; ok {
		return n
	}
	if o.index == nil {
		o.index = map[string]int32{}
	}
	n := int32(len(o.sets))
	o.sets = append(o.sets, slices.Clone(ids))
	o.index[string(key)] = n
	return n
}

func (o *objSets) union(a, b int32) int32 {
	if a == b {
		return a
	}
	if a > b {
		a, b = b, a
	}
	if n, ok := o.merge[[2]int32{a, b}]; ok {
		return n
	}
	if o.merge == nil {
		o.merge = map[[2]int32]int32{}
	}
	n := o.of(append(slices.Clone(o.sets[a]), o.sets[b]...)...)
	o.merge[[2]int32{a, b}] = n
	return n
}

func (o *objSets) held(set int32) int32 {
	var keep []int32
	for _, x := range o.sets[set] {
		if x >= 0 && objState(x) == objLive {
			keep = append(keep, x)
		}
	}
	if len(keep) == 0 || len(keep) == len(o.sets[set]) {
		return set
	}
	return o.of(keep...)
}
