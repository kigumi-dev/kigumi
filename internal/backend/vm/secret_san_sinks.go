package vm

import "kigumi/internal/compiler/sem"

// secretPrim is a std/secret function the checker knows by its declaration.
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
	if e.Kind != sem.EntFn || m.r.Packages[e.Pkg].Path != "std/secret" {
		return primNone
	}
	switch e.Name {
	case "classify":
		return primClassify
	case "declassify", "declassifyBytes":
		return primDeclassify
	}
	return primNone
}

// postCall labels what classify returns and clears what declassify returns; callPrim labels every other primitive.
func (m *Machine) postCall(ent sem.EntityID, res *obj) *obj {
	if !m.opts.SecretSan {
		return res
	}
	switch m.secretPrimOf(ent) {
	case primClassify:
		return m.withTaint(res, secretCoarse)
	case primDeclassify:
		return m.untaint(res)
	}
	return res
}

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

// checkBranch covers `if`, `match` and loop conditions, which all lower to TermBranch.
func (m *Machine) checkBranch(cond *obj) {
	if m.opts.SecretSan {
		m.secretSink("a conditional branch", cond)
	}
}

func (m *Machine) checkIndex(idx *obj) {
	if m.opts.SecretSan {
		m.secretSink("an index", idx)
	}
}

func (m *Machine) checkAddress(addr *obj) {
	if m.opts.SecretSan {
		m.secretSink("an address", addr)
	}
}

func (m *Machine) checkAllocSize(n *obj, what string) {
	if m.opts.SecretSan {
		m.secretSink("an allocation size ("+what+")", n)
	}
}

func (m *Machine) checkHostIO(args []*obj, what string) {
	if m.opts.SecretSan {
		m.secretSinkShadow("host I/O ("+what+")", m.deepTaint(args))
	}
}

// checkBinopSinks refuses the operands an operator decides on: a checked add/sub/mul or `<<` tests overflow,
// a shift tests its count's range, and div/rem take value-dependent time.
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
	case bopShl, bopShr:
		m.secretSink("a shift count", b)
		if nk := deref(a).nkind(); c == bopShl && (is128(nk) || isSigned(nk) || bitsOf(nk) < 64) {
			m.secretSink("an overflow check", a)
		}
	}
}
