package vm

// minArrayCap is the smallest backing store native gives a non-empty array literal.
const minArrayCap = 4

// capRule is how one std primitive treats an array's capacity on native: born sizes the store of the array it creates
// (nil when it creates none) and extra is what it asks try_grow for (nil when it never grows one).
type capRule struct {
	born  func(n int64) int64
	extra func(arg *obj) uint64
}

func literalCap(n int64) int64 {
	if n == 0 {
		return 0
	}
	return max(n, minArrayCap)
}

func exactCap(n int64) int64 { return n }

func one(*obj) uint64 { return 1 }

// arrayCapRules lists every primitive that sizes an array, keyed like the native std op table.
// Arrays made outside it (literals, copies, slices) are rt_array's and take literalCap.
var arrayCapRules = map[string]capRule{
	"array.Array.empty":           {born: literalCap},
	"array.Array.tryWithCapacity": {born: exactCap},
	"array.Array.tryMap":          {born: exactCap},
	"array.Array.tryFilter":       {born: exactCap, extra: one},
	"array.Array.trySlice":        {born: exactCap},
	"map.Map.tryEntries":          {born: exactCap},
	"array.Array.push":            {extra: one},
	"array.Array.pushCopy":        {extra: one},
	"array.Array.tryInsertAt":     {extra: one},
	"array.Array.growBy":          {extra: func(n *obj) uint64 { return uint64(n.i) }},
}

func mkArray(items []*obj) *obj {
	v := withFields(kArray, 0, items)
	v.i = literalCap(int64(len(items)))
	return v
}

// newArray is the array key's primitive returns for items, its store sized for size slots as native would.
func newArray(key string, size int64, items []*obj) *obj {
	v := withFields(kArray, 0, items)
	v.i = max(arrayCapRules[key].born(size), int64(len(items)))
	return v
}

// birthRefused takes the refusable allocations of creating an n-slot array for key: the header, then the store when it is not empty.
func (m *Machine) birthRefused(key string, n int64) bool {
	if m.faultTick() {
		return true
	}
	return arrayCapRules[key].born(n) > 0 && m.faultTick()
}

// growFor makes room as key's primitive does on native: the refusable allocation is taken only when the modelled
// capacity is exceeded, and a refusal leaves the capacity as it was.
func (m *Machine) growFor(key string, a, arg *obj) bool {
	need := uint64(len(a.fields)) + arrayCapRules[key].extra(arg)
	capacity := uint64(max(a.i, int64(len(a.fields))))
	if need > capacity {
		if m.faultTick() {
			return false
		}
		capacity = max(capacity*2, need)
	}
	a.i = int64(capacity)
	return true
}
