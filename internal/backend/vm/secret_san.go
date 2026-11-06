package vm

import (
	"sync/atomic"

	"kigumi/internal/compiler/sem"
)

// SecretSan is the opt-in constant-time shadow checker described in M4 of
// docs/secret_constant_time_plan_2026-10-15.md.
//
// A shadow labels a value the program observed from a secret source. The only
// source recognised by the checker is the declaration identity of the
// std/secret primitives: the value returned by `SecretU8/16/32/64.classify`
// is shadowed, and `declassify` removes the shadow. No type name is inspected:
// the label follows values, not their static type, so ordinary records and
// integers that happen to be named "Secret" are not special-cased.
//
// Integer shadows are bit-precise for bitwise operations, so AND with a public
// zero clears the shadow and OR with a public zero keeps it, as MSan does.
// Arithmetic joins conservatively: if either operand is shadowed the result is
// shadowed in every bit of the destination width. Comparisons, floats and
// aggregates are shadowed coarsely (any secret dependence taints the whole
// value), because bit precision has no meaning for them.
//
// A shadowed value reaching a sink aborts the run with a "SecretSan:"
// diagnostic. The sinks wired to VM instructions are:
//
//   - a conditional branch, which every `if`, `match` and loop condition
//     lowers to (frame.run and the async resume loop check TermBranch);
//   - an index, slice bound or array/bytes element access: OpIndex and
//     OpSetIndex, plus the library primitives that take an index
//     (array.Array.get/with/withMut/takeAt/trySlice, prelude.Bytes.get/
//     trySlice, prelude.String.trySliceBytes);
//   - an allocation size: prelude.Bytes.tryZeros and ByteBuffer.tryNew
//     (which Bytes.tryFill, ByteBuffer and Bytes.zeros go through),
//     array.Array.tryWithCapacity, and array.Array.growBy;
//   - a division or remainder operand;
//   - a checked add/sub/mul/negate, whose overflow decision branches on the
//     operands (`wrapping*` primitives are the constant-time alternative);
//   - a host effect (hostEffect's I/O prefixes), and the print/eprint/panic
//     builtins and OpShell, whose arguments are observed outside the program.
//
// Not yet wired, and therefore not silently trusted:
//
//   - inline asm (OpAsm): its inputs and output carry no shadow, because the
//     checker cannot see the asm body. M3 covers the generated code.
//   - RC decisions: a release happens on ownership, never on a shadowed
//     operand, so there is no direct check; the two-run trace compares the
//     RC operation sequence instead.
//   - array/map growth that infers a size from a container's public length
//     (push/concat/map insert), and Map lookups keyed by a secret: the
//     constant-time discipline keeps those public; a secret key is not yet
//     flagged.
//   - a host primitive's return value is not a taint source (entropy, clock,
//     environment). A secret built from one is tagged by classify.
//
// The two-run mode (secret_san_trace.go) compares executed blocks,
// object-relative indices, allocation events and RC operations between the
// same function run on two secret inputs.

// shadow is a value's SecretSan label. lo/hi carry a per-bit dependence mask
// for integer values; coarse marks a dependence that is not tracked bit by
// bit (a Bool, an aggregate, a float or a string).
type shadow struct {
	lo, hi uint64
	coarse bool
}

// tainted reports whether the label says anything at all.
func (s shadow) tainted() bool { return s.coarse || s.lo != 0 || s.hi != 0 }

// merge joins two labels: a value that depends on either is shadowed.
func (s shadow) merge(o shadow) shadow {
	return shadow{lo: s.lo | o.lo, hi: s.hi | o.hi, coarse: s.coarse || o.coarse}
}

// secretCoarse is the label of a value whose whole content is secret.
var secretCoarse = shadow{coarse: true}

// fullShadow labels every bit of an integer of kind nk.
func fullShadow(nk int) shadow {
	if is128(nk) {
		return shadow{lo: ^uint64(0), hi: ^uint64(0)}
	}
	if b := bitsOf(nk); b < 64 {
		return shadow{lo: uint64(1)<<b - 1}
	}
	return shadow{lo: ^uint64(0)}
}

// maskTo trims a bit shadow to the width of nk.
func maskTo(s shadow, nk int) shadow {
	if is128(nk) {
		return s
	}
	if b := bitsOf(nk); b < 64 {
		m := uint64(1)<<b - 1
		return shadow{lo: s.lo & m, hi: 0, coarse: s.coarse}
	}
	return shadow{lo: s.lo, hi: 0, coarse: s.coarse}
}

// labelOf reads the label table entry of one value.
func (m *Machine) labelOf(v *obj) shadow {
	m.sh.shadowMu.Lock()
	t := m.sh.shadows[v]
	m.sh.shadowMu.Unlock()
	return t
}

// addLabel joins t into v's label.
func (m *Machine) addLabel(v *obj, t shadow) {
	if v == nil || !t.tainted() {
		return
	}
	m.sh.shadowMu.Lock()
	if m.sh.shadows == nil {
		m.sh.shadows = map[*obj]shadow{}
	}
	m.sh.shadows[v] = m.sh.shadows[v].merge(t)
	m.sh.shadowMu.Unlock()
}

// replaceLabel sets v's label, dropping the entry when t is empty.
func (m *Machine) replaceLabel(v *obj, t shadow) {
	if v == nil {
		return
	}
	m.sh.shadowMu.Lock()
	if t.tainted() {
		if m.sh.shadows == nil {
			m.sh.shadows = map[*obj]shadow{}
		}
		m.sh.shadows[v] = t
	} else {
		delete(m.sh.shadows, v)
	}
	m.sh.shadowMu.Unlock()
}

// clearLabel forgets v's label, as a freed value must not lend it to the next
// occupant of the same address.
func (m *Machine) clearLabel(v *obj) {
	if v == nil {
		return
	}
	m.sh.shadowMu.Lock()
	delete(m.sh.shadows, v)
	m.sh.shadowMu.Unlock()
}

// taintOf reads the label of v, following cell and box wrappers to the value
// they hold so a secret stored in a Box keeps its label.
func (m *Machine) taintOf(v *obj) shadow {
	if !m.opts.SecretSan {
		return shadow{}
	}
	var t shadow
	for v != nil {
		t = t.merge(m.labelOf(v))
		switch v.k {
		case kCell, kBox:
			v = v.inner
		default:
			return t
		}
	}
	return t
}

// joinTaint is the label of a value derived from all of args.
func (m *Machine) joinTaint(args []*obj) shadow {
	var t shadow
	for _, a := range args {
		t = t.merge(m.taintOf(a))
	}
	return t
}

// withTaint labels v with t and returns the labeled value. A value the lane
// does not own (an interned scalar or a static object) is detached first, so
// one run never rewrites a label another run or another name can see.
func (m *Machine) withTaint(v *obj, t shadow) *obj {
	if !m.opts.SecretSan || v == nil || !t.tainted() {
		return v
	}
	if atomic.LoadInt32(&v.rc) < 0 {
		// An immortal is shared with other names or runs: derive a value of
		// its own instead of relabelling the shared one.
		c := m.detachValue(v)
		if c == nil {
			return v
		}
		v = c
	}
	m.addLabel(v, t)
	return v
}

// inheritLabel gives a runtime copy dst the label of the value it copied.
func (m *Machine) inheritLabel(src, dst *obj) *obj {
	if !m.opts.SecretSan || src == nil || dst == nil || src == dst {
		return dst
	}
	if t := m.labelOf(src); t.tainted() {
		m.addLabel(dst, t)
	}
	return dst
}

// deriveTaint returns a value that carries res's content plus src's label. It
// copies res so clearing the copy's label later (declassify) cannot clear the
// label of the value res came from.
func (m *Machine) deriveTaint(res *obj, src shadow) *obj {
	if !m.opts.SecretSan || res == nil || !src.tainted() {
		return res
	}
	c := m.detachValue(res)
	if c == nil || c == res {
		m.addLabel(res, src)
		return res
	}
	m.release(res)
	m.addLabel(c, src)
	return c
}

// detachValue copies v into a value this run may label without touching v.
// It returns nil when v cannot be detached (a shared resource or an opaque
// handle); the caller then labels v in place.
func (m *Machine) detachValue(v *obj) *obj {
	switch v.k {
	case kInt:
		n := newObj(kInt)
		n.i, n.w, n.numk = v.i, v.w, v.numk
		return n
	case kFloat:
		n := newObj(kFloat)
		n.i, n.numk = v.i, v.numk
		return n
	case kBool:
		n := newObj(kBool)
		n.b = v.b
		return n
	case kChar:
		n := newObj(kChar)
		n.i = v.i
		return n
	case kStr, kBytes:
		n := newObj(v.k)
		n.setStr(append([]byte(nil), v.str()...))
		m.chargeMemory(n)
		return n
	case kUnit:
		return nil
	case kRecord:
		if m.isResource(v.ent) {
			return nil
		}
	case kClosure, kOpaque, kCell, kRawArray:
		return nil
	}
	c := m.copy(v)
	if c == v {
		return nil
	}
	return c
}

// untaint clears a label from a value this run just produced (declassify).
func (m *Machine) untaint(v *obj) *obj {
	if v != nil && atomic.LoadInt32(&v.rc) >= 0 {
		m.clearLabel(v)
	}
	return v
}

// secretPrim identifies the std/secret primitives the checker knows by their
// declaration identity. It never looks at a type's spelling.
type secretPrim uint8

const (
	primNone secretPrim = iota
	primClassify
	primDeclassify
)

func (m *Machine) secretPrimOf(ent sem.EntityID) secretPrim {
	if ent == 0 || m.r == nil || int(ent) >= len(m.r.Entities) {
		return primNone
	}
	e := m.r.Entity(ent)
	if e.Kind != sem.EntFn {
		return primNone
	}
	if m.r.Packages[e.Pkg].Path != "std/secret" {
		return primNone
	}
	switch e.Name {
	case "classify":
		return primClassify
	case "declassify":
		return primDeclassify
	}
	return primNone
}

// stdResultTaint is the label of a bodiless std primitive's result. Most
// primitives are opaque to the checker, so the arguments' labels join; a
// select is precise, because a bit where both arms agree does not depend on
// the secret mask.
func (m *Machine) stdResultTaint(key string, args []*obj, r *obj) *obj {
	if !m.opts.SecretSan {
		return r
	}
	if key == "secret.selectU64" && len(args) >= 3 && m.taintOf(args[0]).tainted() {
		return m.withTaint(r, m.selectShadow(args[0], args[1], args[2]))
	}
	return m.withTaint(r, m.joinTaint(args))
}

// selectShadow is the label of (m ? a : b) at bit granularity: a bit where a
// and b hold the same value is independent of m, so a public zero in both
// arms clears it (M4's "public select" rule).
func (m *Machine) selectShadow(mask, a, b *obj) shadow {
	tm, ta, tb := m.taintOf(mask), m.taintOf(a), m.taintOf(b)
	av, bv := widen(deref(a)), widen(deref(b))
	eq := wide{lo: ^(av.lo ^ bv.lo), hi: ^(av.hi ^ bv.hi)}
	t := shadow{
		lo: (ta.lo|tb.lo)&eq.lo | (tm.lo|ta.lo|tb.lo)&^eq.lo,
		hi: (ta.hi|tb.hi)&eq.hi | (tm.hi|ta.hi|tb.hi)&^eq.hi,
	}
	if tm.coarse || ta.coarse || tb.coarse {
		t.coarse = true
	}
	return t
}

// postCall applies the checker to a call result: classify shadows what it
// returns, declassify clears the label, and a primitive with no Kigumi body
// (whose semantics the checker cannot follow) propagates its arguments'
// labels conservatively. A lowered body propagates labels itself.
func (m *Machine) postCall(ent sem.EntityID, args []*obj, res *obj, bodyRan bool) *obj {
	if !m.opts.SecretSan {
		return res
	}
	switch m.secretPrimOf(ent) {
	case primClassify:
		return m.withTaint(res, secretCoarse)
	case primDeclassify:
		return m.untaint(res)
	}
	if bodyRan {
		return res
	}
	return m.withTaint(res, m.joinTaint(args))
}

// postCallValue applies the two declaration-identity rules to a static or
// witness call. Other value calls propagate through the body itself.
func (m *Machine) postCallValue(ent sem.EntityID, res *obj) *obj {
	switch m.secretPrimOf(ent) {
	case primClassify:
		return m.withTaint(res, secretCoarse)
	case primDeclassify:
		return m.untaint(res)
	}
	return res
}

// secretSink aborts when v carries a secret label.
func (m *Machine) secretSink(what string, v *obj) {
	m.secretSinkShadow(what, m.taintOf(v))
}

func (m *Machine) secretSinkShadow(what string, s shadow) {
	if !s.tainted() {
		return
	}
	where := ""
	if m.curFn != nil {
		where = " in " + m.curFn.Name
	}
	m.abort("SecretSan: secret value reaches " + what + where)
}

// checkBranch refuses a conditional whose truth depends on a secret, catching
// `if`, `match` and loop conditions alike since every one lowers to TermBranch.
func (m *Machine) checkBranch(cond *obj) {
	if m.opts.SecretSan {
		m.secretSink("a conditional branch", cond)
	}
}

// checkIndex refuses a secret index, slice bound or address.
func (m *Machine) checkIndex(idx *obj) {
	if m.opts.SecretSan {
		m.secretSink("an index", idx)
	}
}

// checkAllocSize refuses an allocation whose size depends on a secret.
func (m *Machine) checkAllocSize(n *obj, what string) {
	if m.opts.SecretSan {
		m.secretSink("an allocation size ("+what+")", n)
	}
}

// checkHostIO refuses a host effect (I/O) argument that carries a secret.
func (m *Machine) checkHostIO(args []*obj, what string) {
	if m.opts.SecretSan {
		m.secretSinkShadow("host I/O ("+what+")", m.joinTaint(args))
	}
}

// checkBinopSinks refuses the value-dependent decisions of an arithmetic
// operator: a checked add/sub/mul (the overflow branch), and div/rem (whose
// timing depends on the operands on many targets).
func (m *Machine) checkBinopSinks(c bop, a, b *obj) {
	if !m.opts.SecretSan {
		return
	}
	switch c {
	case bopDiv, bopRem:
		m.secretSink("a division operand", a)
		m.secretSink("a division operand", b)
	case bopAdd, bopSub, bopMul:
		m.secretSink("an overflow check", a)
		m.secretSink("an overflow check", b)
	}
}

// binopShadow is the label a binary operation result carries.
func (m *Machine) binopShadow(c bop, op string, a, b *obj) shadow {
	ta, tb := m.taintOf(a), m.taintOf(b)
	if !ta.tainted() && !tb.tainted() {
		return shadow{}
	}
	da, db := deref(a), deref(b)
	switch c {
	case bopEq, bopNe, bopLt, bopLe, bopGt, bopGe:
		return secretCoarse
	case bopAnd, bopOr, bopXor:
		if da.k == kInt && db.k == kInt {
			return maskTo(m.bitShadow(c, ta, tb, widen(da), widen(db)), da.nkind())
		}
		return secretCoarse
	case bopShl, bopShr:
		if tb.tainted() {
			// A secret shift count is not one of M4's sinks, so the result is
			// conservatively shadowed in every bit instead of failing.
			return fullShadow(da.nkind())
		}
		return maskTo(m.shiftShadow(c, ta, db, da.nkind()), da.nkind())
	case bopOther:
		return secretCoarse
	}
	// Add, sub, mul and the like: every bit may depend on either operand.
	return fullShadow(da.nkind())
}

// bitShadow is the precise MSan rule for &, | and ^: a result bit depends on
// an operand bit only where the other operand lets it through.
func (m *Machine) bitShadow(c bop, ta, tb shadow, av, bv wide) shadow {
	switch c {
	case bopAnd:
		return shadow{lo: ta.lo&bv.lo | tb.lo&av.lo, hi: ta.hi&bv.hi | tb.hi&av.hi}
	case bopOr:
		return shadow{lo: ta.lo&^bv.lo | tb.lo&^av.lo, hi: ta.hi&^bv.hi | tb.hi&^av.hi}
	default: // bopXor: every result bit depends on both operands
		return shadow{lo: ta.lo | tb.lo, hi: ta.hi | tb.hi}
	}
}

// shiftShadow shifts a bit label with its value; the shift count is public
// here (a shadowed count was handled conservatively by the caller).
func (m *Machine) shiftShadow(c bop, ta shadow, count *obj, nk int) shadow {
	n := uint(uint64(count.i))
	if c == bopShl {
		if is128(nk) {
			return wideShadow(shlW(wide{ta.lo, ta.hi}, n))
		}
		return shadow{lo: ta.lo << n}
	}
	if is128(nk) {
		s := wideShadow(shrW(wide{ta.lo, ta.hi}, n))
		if isSigned(nk) && ta.hi>>63 != 0 && n > 0 {
			// An arithmetic shift fills with the (possibly secret) sign.
			s.hi |= ^uint64(0) << (64 - n)
		}
		return s
	}
	s := shadow{lo: ta.lo >> n}
	if isSigned(nk) && ta.lo>>63 != 0 && n > 0 {
		s.lo |= ^uint64(0) << (64 - n)
	}
	return s
}

func wideShadow(w wide) shadow { return shadow{lo: w.lo, hi: w.hi} }

// unopShadow is the label a unary operation result carries.
func (m *Machine) unopShadow(op string, a *obj) shadow {
	ta := m.taintOf(a)
	if !ta.tainted() {
		return shadow{}
	}
	switch op[0] {
	case '!':
		return secretCoarse
	case '~':
		da := deref(a)
		if da.k == kInt {
			return maskTo(ta, da.nkind())
		}
		return secretCoarse
	default: // '-'
		da := deref(a)
		if da.k == kInt {
			return fullShadow(da.nkind())
		}
		return secretCoarse
	}
}

// castShadow carries a bit label across a numeric conversion.
func (m *Machine) castShadow(a *obj, nk int) shadow {
	ta := m.taintOf(a)
	if !ta.tainted() {
		return shadow{}
	}
	if nk&512 != 0 { // float
		return secretCoarse
	}
	da := deref(a)
	if da.k != kInt {
		return secretCoarse
	}
	return maskTo(ta, nk)
}
