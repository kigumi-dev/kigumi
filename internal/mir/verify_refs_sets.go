package mir

import "slices"

type objSets struct {
	sets    [][]int32
	index   map[string]int32
	merge   map[[2]int32]int32
	one     []int32
	objects []bool
}

func newObjSets(objects int) *objSets {
	return &objSets{one: make([]int32, objects)}
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
	o.objects = append(o.objects, len(ids) > 0 && ids[len(ids)-1] >= 0)
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
