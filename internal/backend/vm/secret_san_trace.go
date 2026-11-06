package vm

import (
	"fmt"
	"io"
	"strings"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
)

// The two-run mode watches a function that runs twice with the same public
// inputs and two different secret inputs. It does not look at values, only at
// the shape of the computation: the blocks executed, the object-relative
// indices touched, the allocation events and the RC operations. Any difference
// between the two traces is a leak of the secret into an observable decision.
//
// This is Microwalk/DATA's idea done at the MIR level, so no binary tracer is
// needed: the VM records what it is about to execute.

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

// traceSize is the byte size of a value's payload, the number the trace
// compares so a secret-sized allocation cannot hide behind an equal count.
// It descends into aggregates so the size of a Bytes inside an Ok is visible.
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

// traceBlock records entering one block.
func (m *Machine) traceBlock(f *midir.Func, bid midir.BlockID) {
	m.trace.block(f, bid)
}

// traceIndex records one object-relative index access.
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

// diff compares two runs and names the first difference.
func (t *traceLog) diff(o *traceLog) error {
	if t == nil || o == nil {
		return nil
	}
	n := min(len(t.blocks), len(o.blocks))
	for i := range t.blocks[:n] {
		a, b := t.blocks[i], o.blocks[i]
		if a != b {
			return fmt.Errorf("SecretSan: trace mismatch in executed blocks at step %d: %s#%d vs %s#%d", i, a.fn, a.blk, b.fn, b.blk)
		}
	}
	if len(t.blocks) != len(o.blocks) {
		return fmt.Errorf("SecretSan: trace mismatch in executed blocks: %d steps vs %d", len(t.blocks), len(o.blocks))
	}
	n = min(len(t.indices), len(o.indices))
	for i := range t.indices[:n] {
		a, b := t.indices[i], o.indices[i]
		if a != b {
			return fmt.Errorf("SecretSan: trace mismatch in object-relative indices at step %d: %s %s[%d] vs %s %s[%d]",
				i, a.op, kindName(a.kind), a.index, b.op, kindName(b.kind), b.index)
		}
	}
	if len(t.indices) != len(o.indices) {
		return fmt.Errorf("SecretSan: trace mismatch in object-relative indices: %d accesses vs %d", len(t.indices), len(o.indices))
	}
	n = min(len(t.allocs), len(o.allocs))
	for i := range t.allocs[:n] {
		a, b := t.allocs[i], o.allocs[i]
		if a != b {
			return fmt.Errorf("SecretSan: trace mismatch in allocations at step %d: %s of %d bytes vs %s of %d bytes",
				i, kindName(a.kind), a.size, kindName(b.kind), b.size)
		}
	}
	if len(t.allocs) != len(o.allocs) {
		return fmt.Errorf("SecretSan: trace mismatch in allocations: %d events vs %d", len(t.allocs), len(o.allocs))
	}
	n = min(len(t.rc), len(o.rc))
	for i := range t.rc[:n] {
		a, b := t.rc[i], o.rc[i]
		if a != b {
			return fmt.Errorf("SecretSan: trace mismatch in RC operations at step %d: %s %s vs %s %s",
				i, a.op, kindName(a.kind), b.op, kindName(b.kind))
		}
	}
	if len(t.rc) != len(o.rc) {
		return fmt.Errorf("SecretSan: trace mismatch in RC operations: %d operations vs %d", len(t.rc), len(o.rc))
	}
	return nil
}

// ShadowValue is the label of the value a SecretSanRun call returned.
type ShadowValue struct {
	// Any is a coarse secret dependence (Bool, aggregate, float, string).
	Any bool
	// Lo and Hi are the bit masks for an integer's secret-dependent bits.
	Lo, Hi uint64
}

// Tainted reports whether any part of the value depends on a secret.
func (v ShadowValue) Tainted() bool { return v.Any || v.Lo != 0 || v.Hi != 0 }

// SecretSanRun runs the top-level function named fn with integer arguments,
// marking the positions listed in secret as secret inputs, and returns the
// label of its result. It is the test/driver hook that makes the sinks
// reachable from a program that has no `unsafe` escape: the secret positions
// stand in for a value a calling context knows to be secret, exactly as the
// two-run mode's secret inputs do.
//
// Options.SecretSan is forced on. The function must be lowered into prog
// (calling it from the entry, even with public values, is enough).
func SecretSanRun(prog *mir.Program, opts Options, fn string, vals []int64, secret []bool, out, errOut io.Writer) (ShadowValue, error) {
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = io.Discard
	}
	opts.SecretSan = true
	opts.Trace = false
	m, f, args, err := runFunction(prog, opts, fn, vals, secret, out, errOut)
	if err != nil {
		return ShadowValue{}, err
	}
	var res *obj
	err = m.catch(func() { res = m.call(f, args) })
	if err != nil {
		return ShadowValue{}, err
	}
	s := m.taintOf(res)
	return ShadowValue{Any: s.coarse, Lo: s.lo, Hi: s.hi}, nil
}

// CompareTraces runs fn once with a and once with b under the two-run trace
// and reports the first difference. a and b must be the same length: the test
// keeps the public positions equal and varies only the secret ones.
// It is the test/driver hook the M4 acceptance uses to assert equality and,
// with a deliberately leaking function, difference.
func CompareTraces(prog *mir.Program, opts Options, fn string, a, b []int64, out, errOut io.Writer) error {
	if len(a) != len(b) {
		return fmt.Errorf("SecretSan: trace comparison needs equal-length input vectors, got %d and %d", len(a), len(b))
	}
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = io.Discard
	}
	traces := make([]*traceLog, 2)
	for i, vals := range [][]int64{a, b} {
		o := opts
		o.SecretSan = false
		o.Trace = true
		m, f, args, err := runFunction(prog, o, fn, vals, nil, out, errOut)
		if err != nil {
			return err
		}
		if err := m.catch(func() { m.call(f, args) }); err != nil {
			return err
		}
		traces[i] = m.trace
	}
	return traces[0].diff(traces[1])
}

// runFunction resolves fn and builds its integer arguments in a fresh
// machine's value model.
func runFunction(prog *mir.Program, opts Options, fn string, vals []int64, secret []bool, out, errOut io.Writer) (*Machine, *midir.Func, []*obj, error) {
	m := newMachine(prog, opts, out, errOut)
	var f *midir.Func
	for _, cand := range prog.Funcs {
		if cand.Name == fn || strings.HasSuffix(cand.Name, "."+fn) {
			f = cand
			break
		}
	}
	if f == nil {
		return nil, nil, nil, fmt.Errorf("SecretSan: no lowered function named %q", fn)
	}
	if secret != nil && len(secret) != len(vals) {
		return nil, nil, nil, fmt.Errorf("SecretSan: %d secret marks for %d arguments", len(secret), len(vals))
	}
	args, err := m.intArgs(f, vals)
	if err != nil {
		return nil, nil, nil, err
	}
	for i := range args {
		if secret != nil && i < len(secret) && secret[i] {
			nk := m.numKind(f.Locals[f.Params[i]].Type)
			args[i] = m.withTaint(args[i], fullShadow(nk))
		}
	}
	return m, f, args, nil
}

// intArgs builds one value per parameter.
func (m *Machine) intArgs(f *midir.Func, vals []int64) ([]*obj, error) {
	if len(vals) != len(f.Params) {
		return nil, fmt.Errorf("SecretSan: %s takes %d arguments, got %d", f.Name, len(f.Params), len(vals))
	}
	out := make([]*obj, len(vals))
	for i, p := range f.Params {
		t := f.Locals[p].Type
		if !m.r.Types.IsInteger(m.underlyingRepr(t)) {
			return nil, fmt.Errorf("SecretSan: %s argument %d is not an integer", f.Name, i)
		}
		out[i] = mkInt(vals[i], m.numKind(t))
	}
	return out, nil
}
