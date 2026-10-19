package vm

import (
	"math/bits"
	"slices"
	"strings"
	"sync/atomic"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

// Native's kg.box helper allocates a header and a slot array per aggregate and a box per scalar leaf outside its static tables.
func boxTicks(v *obj) int {
	v = deref(v)
	if v == nil {
		return 0
	}
	switch v.k {
	case kInt, kFloat, kChar:
		if scalarBoxes(v) {
			return 1
		}
		return 0
	case kRecord, kVariant:
		if len(v.fields) == 0 && v.k == kVariant {
			return 0
		}
		return 2 + leafTicks(v.fields)
	case kClosure:
		if v.cenv() == nil || len(v.cenv().fields) == 0 {
			return 2
		}
		return 3 + leafTicks(v.cenv().fields)
	case kArray:
		if len(v.fields) == 0 {
			return 1
		}
		return 1 + bits.Len(uint(len(v.fields)-1)) + 1 + leafTicks(v.fields)
	}
	return 0
}

func leafTicks(fs []*obj) int {
	n := 0
	for _, f := range fs {
		n += boxTicks(f)
	}
	return n
}

// scalarBoxes reports whether native allocates for scalar v: small integers below 128 bits and ASCII chars are static there.
func scalarBoxes(v *obj) bool {
	switch v.k {
	case kInt:
		return uint64(v.i-internMin) >= internN || internSlot(v.nkind()) < 0
	case kFloat:
		return true
	case kChar:
		return v.ch() >= 128
	}
	return false
}

func (m *Machine) scalarTick(v *obj) {
	if m.opts.FaultInjectAt <= 0 || v == nil || atomic.LoadInt32(&v.rc) < 0 || !scalarBoxes(v) {
		return
	}
	if m.faultTick() {
		m.abort("out of memory")
	}
}

// A callee handing back an argument returns the box it was given, so no tick.
func (m *Machine) resultTick(v *obj, args []*obj) {
	if slices.Contains(args, v) {
		return
	}
	m.scalarTick(v)
}

// closureCall reports a call native makes straight into an unboxed closure, which still takes and returns scalars boxed.
func (m *Machine) closureCall(f *mir.Func, in *mir.Inst) bool {
	return in.Op == mir.OpCallValue && m.r.Types.Kind(f.Locals[in.Args[0]].Type) == sem.KClosure
}

// A local computed here is boxed by native on its way into a boxed slot; a compile-time value is an immortal on both backends.
type localFacts struct {
	fixed, computed []bool
}

func (m *Machine) factsOf(f *mir.Func) localFacts {
	lf, ok := m.boxFacts[f]
	if ok {
		return lf
	}
	if m.boxFacts == nil {
		m.boxFacts = map[*mir.Func]localFacts{}
	}
	lf = localFacts{fixed: m.p.FixedLocals(f), computed: computedLocals(f)}
	m.boxFacts[f] = lf
	return lf
}

func computedLocals(f *mir.Func) []bool {
	defs := make([]int, len(f.Locals))
	out := make([]bool, len(f.Locals))
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op.Yields() {
				defs[in.Dst]++
				out[in.Dst] = in.Op == mir.OpBinary || in.Op == mir.OpUnary
			}
		}
	}
	for i := range out {
		out[i] = out[i] && defs[i] == 1
	}
	return out
}

// A literal argument is an immortal on native, so it takes no tick.
func (m *Machine) argTicks(f *mir.Func, in *mir.Inst, args []*obj) {
	if m.opts.FaultInjectAt <= 0 || in.Op != mir.OpCallValue || in.Ent != 0 {
		return
	}
	fixed := m.factsOf(f).fixed
	for i, a := range in.Args[1:] {
		if fixed[a] || !m.r.ScalarBoxes(f.Locals[a].Type) || !scalarBoxes(args[i+1]) {
			continue
		}
		if m.faultTick() {
			m.abort("out of memory")
		}
	}
}

func (m *Machine) storeTick(f *mir.Func, in *mir.Inst, l mir.LocalID, v *obj) {
	if m.opts.FaultInjectAt <= 0 || in.Op != mir.OpCellSet && f.IsDirect(in.Args[0]) || !m.factsOf(f).computed[l] {
		return
	}
	m.scalarTick(v)
}

// stdHash reports a std hash primitive called through a value: native computes it in a register (rt_callv_i64).
func stdHash(fv *obj) bool {
	fv = deref(fv)
	if fv == nil || fv.k != kClosure || fv.creq() != nil || fv.async || len(fv.cenv().fields) != 0 {
		return false
	}
	key := fv.cstd()
	return key == "hash" || strings.HasSuffix(key, ".hash")
}
