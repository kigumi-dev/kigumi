package borrow

import (
	"slices"
)

// witnessEntry is a witnessSet answer, ok false when some use cannot be followed.
type witnessEntry struct {
	fns []boundFn
	ok  bool
}

// witnessRounds and witnessNodes bound a cycle whose callers wrap their own witness again on every pass.
const (
	witnessRounds = 16
	witnessNodes  = 256
)

// witnessSolve is witnessReach memoized per key: a cycle of fns passing their witnesses on starts from nothing and is
// rerun until no answer in it changes, every answer in it unknown when that does not settle.
func (c *borrowCheck) witnessSolve(key witnessKey) *witnessEntry {
	m := &c.state.refKeeps
	if en, ok := m.witnesses[key]; ok {
		if slot, open := m.wscc.slot(key); open {
			m.wlow = min(m.wlow, slot)
		}
		return en
	}
	if m.witnesses == nil {
		m.witnesses = map[witnessKey]*witnessEntry{}
	}
	en := &witnessEntry{ok: true}
	m.witnesses[key] = en
	slot := m.wscc.push(key)
	outer := m.wlow
	m.wlow = noCut
	en.fns, en.ok = c.witnessReach(key)
	low := m.wlow
	if low < slot {
		m.wlow = min(outer, low)
		return en
	}
	c.settleWitnesses(slot, low == slot)
	m.wlow = outer
	return en
}

func (c *borrowCheck) settleWitnesses(slot int, recursive bool) {
	m := &c.state.refKeeps
	for round := 0; recursive; round++ {
		members := m.wscc.members(slot)
		changed, grown := false, false
		for _, k := range slices.Backward(members) {
			en := m.witnesses[k]
			fns, ok := c.witnessReach(k)
			if ok != en.ok || (fnBind{0: fns}).key() != (fnBind{0: en.fns}).key() {
				en.fns, en.ok, changed = fns, ok, true
			}
			grown = grown || fnsSize(fns) > witnessNodes
		}
		if !changed && len(m.wscc.members(slot)) == len(members) {
			break
		}
		if grown || round == witnessRounds {
			for _, k := range m.wscc.members(slot) {
				m.witnesses[k].fns, m.witnesses[k].ok = nil, false
			}
			break
		}
	}
	for _, k := range m.wscc.pop(slot) {
		c.probeWitness(k, m.witnesses[k])
	}
}

func (c *borrowCheck) probeWitness(key witnessKey, en *witnessEntry) {
	if witnessProbe == nil || !en.ok {
		return
	}
	var wide []boundFn
	for _, impl := range c.implementers(c.p.R.Fn(key.fn).Witnesses[key.w].Req) {
		wide = append(wide, boundFn{ent: impl})
	}
	witnessProbe(c, slices.Clone(en.fns), wide)
}

// uniqueFns drops the repeats of fns, each kept by its first place.
func uniqueFns(fns []boundFn) []boundFn {
	var out []boundFn
	seen := map[string]bool{}
	for _, f := range fns {
		k := fnBind{0: {f}}.key()
		if !seen[k] {
			seen[k] = true
			out = append(out, f)
		}
	}
	return out
}

func fnsSize(fns []boundFn) int {
	n := len(fns)
	for _, f := range fns {
		for _, sub := range f.sub {
			n += fnsSize(sub)
		}
	}
	return n
}
