package vm

import (
	"slices"

	"kigumi/internal/mir"
)

// A crossing is settled when its consumer runs, so a panic leaves the reference the consumer would have dropped: the source when it keeps the copy, the copy when it only reads it.
func (m *Machine) sweepCrossings(fr *frame) {
	insts := fr.f.Blocks[fr.blk].Insts
	for _, c := range fr.pending {
		for j := fr.pc; j < len(insts); j++ {
			in := &insts[j]
			i := slices.Index(in.Args, c.dst)
			if i < 0 {
				continue
			}
			switch {
			case in.Op == mir.OpShare:
				m.sweepCrossed(fr, c.dst, c.copy)
			case m.p.ArgConsumed(fr.f, *in, i):
				m.sweepCrossed(fr, c.src, c.from)
			case j == fr.pc:
				m.sweepCrossed(fr, c.dst, c.copy)
			}
			break
		}
	}
	fr.pending = fr.pending[:0]
}

// A local the set sweep already released no longer holds v.
func (m *Machine) sweepCrossed(fr *frame, l mir.LocalID, v *obj) {
	if fr.locals[l] != v {
		return
	}
	fr.locals[l] = nil
	m.sweepRelease(v)
}
