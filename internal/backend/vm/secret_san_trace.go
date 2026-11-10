package vm

import midir "kigumi/internal/compiler/mir/ir"

// The two-run mode runs one function on two secret inputs and compares the shape of the runs, not their values:
// executed blocks, object-relative indices (every one an OpIndex, OpSetIndex, field or primitive index argument), allocations and RC operations.

// traceLog is one run's record.
type traceLog struct {
	blocks  []traceBlock
	indices []traceIndex
	allocs  []traceAlloc
	rc      []traceRC
}

type traceBlock struct {
	fn  string
	blk midir.BlockID
}

type traceIndex struct {
	op    string
	kind  kind
	index int64
}

type traceAlloc struct {
	kind kind
	size int64
}

type traceRC struct {
	op   string
	kind kind
}

func (t *traceLog) block(f *midir.Func, bid midir.BlockID) {
	if t != nil {
		t.blocks = append(t.blocks, traceBlock{fn: f.Name, blk: bid})
	}
}

// decision records the outcome of a branch Go code takes on a callback's result, as a pseudo-block.
func (t *traceLog) decision(r *obj) {
	if t == nil {
		return
	}
	d := deref(r)
	outcome := midir.BlockID(d.ent)
	if d.k == kBool && d.b {
		outcome = 1
	}
	t.blocks = append(t.blocks, traceBlock{fn: "callback branch", blk: outcome})
}

func (t *traceLog) index(op string, base *obj, i int64) {
	if t == nil {
		return
	}
	k := kFree
	if base != nil {
		k = derefRaw(base).k
	}
	t.indices = append(t.indices, traceIndex{op: op, kind: k, index: i})
}

func (t *traceLog) alloc(v *obj) {
	if t == nil || v == nil {
		return
	}
	t.allocs = append(t.allocs, traceAlloc{kind: v.k, size: traceSize(v)})
}

func (t *traceLog) rcOp(op string, v *obj) {
	if t == nil || v == nil {
		return
	}
	t.rc = append(t.rc, traceRC{op: op, kind: v.k})
}

// traceSize is a value's payload size, descending into aggregates so a secret-sized Bytes inside an Ok shows.
func traceSize(v *obj) int64 { return traceSizeAt(v, 0) }

func traceSizeAt(v *obj, depth int) int64 {
	if v == nil || depth > 6 {
		return 8
	}
	switch v.k {
	case kStr, kBytes:
		return int64(len(v.str()))
	case kArray, kRecord, kVariant, kClosure:
		n := int64(8 * len(v.fields))
		for _, f := range v.fields {
			n += traceSizeAt(f, depth+1)
		}
		return n
	case kRawArray:
		return int64(len(v.rawArr().raw)) * 8
	case kCell, kBox:
		return 8 + traceSizeAt(v.inner, depth+1)
	case kOpaque:
		return 8
	}
	return 8
}

func kindName(k kind) string {
	switch k {
	case kFree:
		return "free"
	case kInt:
		return "Int"
	case kFloat:
		return "Float"
	case kBool:
		return "Bool"
	case kStr:
		return "String"
	case kBytes:
		return "Bytes"
	case kChar:
		return "Char"
	case kUnit:
		return "Unit"
	case kRecord:
		return "record"
	case kVariant:
		return "variant"
	case kArray:
		return "array"
	case kClosure:
		return "closure"
	case kBox:
		return "box"
	case kOpaque:
		return "opaque"
	case kCell:
		return "cell"
	case kRawArray:
		return "raw-array"
	}
	return "value"
}

func (m *Machine) traceBlock(f *midir.Func, bid midir.BlockID) {
	m.trace.block(f, bid)
}

func (m *Machine) traceIndex(op string, base *obj, i int64) {
	m.trace.index(op, base, i)
}

// traceIndexAt is traceIndex for an index value object, safe when it is nil.
func (m *Machine) traceIndexAt(op string, base, idx *obj) {
	if m.trace == nil {
		return
	}
	var i int64
	if d := deref(idx); d != nil {
		i = d.i
	}
	m.trace.index(op, base, i)
}
