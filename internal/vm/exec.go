package vm

import (
	"sync/atomic"

	"kigumi/internal/mir"
)

// exec runs one instruction the way llgen lowers it; the forms below read their locals directly and the rest go through execArgs.
func (m *Machine) exec(fr *frame, in *mir.Inst, ic *instCode) {
	f := fr.f
	m.curFn, m.curInst = f, in
	loc := fr.locals
	switch in.Op {
	case mir.OpNop:
		return
	case mir.OpConst:
		if ic.konst != nil {
			loc[in.Dst] = ic.konst
			return
		}
	case mir.OpUnit:
		loc[in.Dst] = unitObj
		return
	case mir.OpAlias, mir.OpBorrow:
		loc[in.Dst] = loc[in.Args[0]]
		return
	case mir.OpMove:
		res := loc[in.Args[0]]
		if f.Locals[in.Args[0]].Ent != 0 {
			loc[in.Args[0]] = nil
		}
		loc[in.Dst] = res
		return
	case mir.OpShare:
		v := loc[in.Args[0]]
		if in.Copy && (!in.IfWritable || m.writableInside(v)) {
			v = m.copy(v)
			m.chargeMemory(v)
		} else {
			v = retain(v)
		}
		loc[in.Dst] = v
		return
	case mir.OpRelease:
		if v := loc[in.Args[0]]; v != nil && atomic.LoadInt32(&v.rc) > 0 {
			m.dropChecked(fr, v)
		}
		return
	case mir.OpDrop:
		v := loc[in.Args[0]]
		if f.Locals[in.Args[0]].Ent != 0 {
			loc[in.Args[0]] = nil
		}
		if v != nil && atomic.LoadInt32(&v.rc) > 0 {
			m.dropChecked(fr, v)
		}
		return
	case mir.OpBinary:
		loc[in.Dst] = m.binopAs(ic.bin, in.Str, loc[in.Args[0]], loc[in.Args[1]])
		return
	case mir.OpUnary:
		if in.Str == "cast" {
			loc[in.Dst] = m.cast(loc[in.Args[0]], ic.nk)
			return
		}
		loc[in.Dst] = m.unop(in.Str, loc[in.Args[0]])
		return
	case mir.OpField:
		loc[in.Dst] = m.field(loc[in.Args[0]], in.Index)
		return
	case mir.OpIsVariant:
		v := deref(loc[in.Args[0]])
		loc[in.Dst] = mkBool(v.k == kVariant && v.ent == in.Ent)
		return
	case mir.OpPayload:
		loc[in.Dst] = retain(deref(loc[in.Args[0]]).fields[in.Index])
		return
	case mir.OpIndex:
		loc[in.Dst] = m.index(loc[in.Args[0]], loc[in.Args[1]])
		return
	case mir.OpCellGet:
		loc[in.Dst] = retain(loc[in.Args[0]].inner)
		return
	case mir.OpCall:
		m.site, m.siteFn = in, f
		if p := m.planFor(in, ic); p.kind == planLowered {
			if m.opts.FaultInjectAt > 0 {
				m.callTicks(f, in, loc)
			}
			loc[in.Dst] = m.callFrom(p.f, p.code, loc, in.Args)
			return
		}
	}
	m.execArgs(fr, in, ic)
}
