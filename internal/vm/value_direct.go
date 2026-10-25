package vm

import (
	"slices"
	"sync/atomic"

	"kigumi/internal/mir"
)

type crossing struct {
	dst, src   mir.LocalID
	from, copy *obj
	hands      bool
}

// cross copies where native copies, so a frame value and a box never share storage.
func (m *Machine) cross(fr *frame, in *mir.Inst, v *obj) *obj {
	if in.Op == mir.OpBoxDirect && !in.Fixed && m.opts.FaultInjectAt > 0 {
		for range m.crossTicks(in.Type, v) {
			if m.faultTick() {
				m.abort("out of memory")
			}
		}
	}
	if m.crossShares(fr, in) {
		return v
	}
	c := m.crossCopy(in.Type, v)
	if fr.f.Locals[in.Dst].Cross {
		fr.pending = append(fr.pending, crossing{dst: in.Dst, src: in.Args[0], from: v, copy: c, hands: m.p.CrossHandsOver(fr.f, in)})
		return c
	}
	fr.locals[in.Args[0]] = nil
	m.release(v)
	return c
}

func (m *Machine) crossShares(fr *frame, in *mir.Inst) bool {
	if !fr.f.Locals[in.Dst].Cross {
		return false
	}
	insts := fr.f.Blocks[fr.blk].Insts
	for j := fr.pc + 1; j < len(insts); j++ {
		r := &insts[j]
		if !slices.Contains(r.Args, in.Dst) {
			continue
		}
		return r.Op == mir.OpAlias && !m.p.CrossHandsOver(fr.f, in) || r.Op == mir.OpShare && r.Copy && !r.IfWritable
	}
	return false
}

// As in native, a handed-over source goes before its consumer runs, so a panic there unwinds the copy with the consumer.
func (m *Machine) settleSources(fr *frame, in *mir.Inst) {
	kept := fr.pending[:0]
	for _, c := range fr.pending {
		i := slices.Index(in.Args, c.dst)
		if i < 0 || !c.hands || in.Op == mir.OpShare || !m.p.ArgConsumed(fr.f, *in, i) {
			kept = append(kept, c)
			continue
		}
		fr.locals[c.src] = nil
		m.release(c.from)
	}
	fr.pending = kept
}

// A consumer keeps the copy so the source is dropped; a reader only borrowed it so the copy is.
func (m *Machine) settle(fr *frame, in *mir.Inst) {
	kept := fr.pending[:0]
	for _, c := range fr.pending {
		i := slices.Index(in.Args, c.dst)
		if i < 0 {
			kept = append(kept, c)
			continue
		}
		named := fr.f.Locals[c.src].Ent != 0
		switch {
		case in.Op == mir.OpShare:
			m.release(c.copy)
		case m.p.ArgConsumed(fr.f, *in, i):
			if in.Op == mir.OpMove && named {
				fr.locals[c.src] = nil
			}
			m.release(c.from)
		default:
			m.release(c.copy)
		}
	}
	fr.pending = kept
}

// A return hands its value to the caller, so its crossing consumes the source.
func (m *Machine) settleReturn(fr *frame, l mir.LocalID) {
	for _, c := range fr.pending {
		if c.dst == l {
			m.release(c.from)
		}
	}
	fr.pending = fr.pending[:0]
}

// Native writes the local's own frame slot, so copy first unless the local is its only holder.
func (m *Machine) ownDirect(fr *frame, l mir.LocalID, v *obj) *obj {
	if v == nil || !fr.f.IsDirect(l) || atomic.LoadInt32(&v.rc) == 1 {
		return v
	}
	c := m.copy(v)
	m.release(v)
	fr.locals[l] = c
	return c
}
