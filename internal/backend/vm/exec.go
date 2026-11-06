package vm

import (
	"sync/atomic"

	midir "kigumi/internal/compiler/mir/ir"
)

// exec runs one instruction the way llgen lowers it; the forms below read their locals directly and the rest go through execArgs.
func (m *Machine) exec(fr *frame, in *midir.Inst, ic *instCode) {
	f := fr.f
	m.curFn, m.curInst = f, in
	loc := fr.locals
	switch in.Op {
	case midir.OpNop:
		return
	case midir.OpConst:
		if ic.konst != nil {
			loc[in.Dst] = ic.konst
			return
		}
	case midir.OpUnit:
		loc[in.Dst] = unitObj
		return
	case midir.OpAlias, midir.OpBorrow:
		loc[in.Dst] = loc[in.Args[0]]
		return
	case midir.OpMove:
		res := loc[in.Args[0]]
		if f.Locals[in.Args[0]].Ent != 0 {
			loc[in.Args[0]] = nil
		}
		loc[in.Dst] = res
		return
	case midir.OpShare:
		v := loc[in.Args[0]]
		if in.Copy && (!in.IfWritable || m.writableInside(v)) {
			m.trace.rcOp("copy", v)
			v = m.copyShared(f, in, ic, v)
			m.chargeMemory(v)
		} else {
			m.trace.rcOp("retain", v)
			v = retain(v)
		}
		loc[in.Dst] = v
		return
	case midir.OpRelease:
		if v := loc[in.Args[0]]; v != nil && atomic.LoadInt32(&v.rc) >= 0 {
			m.dropChecked(fr, v)
		}
		return
	case midir.OpDrop:
		v := loc[in.Args[0]]
		if f.Locals[in.Args[0]].Ent != 0 {
			loc[in.Args[0]] = nil
		}
		if v != nil && atomic.LoadInt32(&v.rc) >= 0 {
			m.dropChecked(fr, v)
		}
		return
	case midir.OpBinary:
		loc[in.Dst] = m.binopAs(ic.bin, in.Str, loc[in.Args[0]], loc[in.Args[1]])
		return
	case midir.OpUnary:
		if in.Str == "cast" {
			loc[in.Dst] = m.cast(loc[in.Args[0]], ic.nk)
			return
		}
		loc[in.Dst] = m.unop(in.Str, loc[in.Args[0]])
		return
	case midir.OpField:
		loc[in.Dst] = m.field(loc[in.Args[0]], in.Index)
		return
	case midir.OpIsVariant:
		v := deref(loc[in.Args[0]])
		loc[in.Dst] = mkBool(v.k == kVariant && v.ent == in.Ent)
		return
	case midir.OpPayload:
		loc[in.Dst] = m.deriveTaint(retain(deref(loc[in.Args[0]]).fields[in.Index]), m.taintOf(loc[in.Args[0]]))
		return
	case midir.OpIndex:
		loc[in.Dst] = m.index(loc[in.Args[0]], loc[in.Args[1]])
		return
	case midir.OpCellGet:
		loc[in.Dst] = m.deriveTaint(retain(loc[in.Args[0]].inner), m.taintOf(loc[in.Args[0]]))
		return
	case midir.OpCall:
		m.site, m.siteFn = in, f
		if p := m.planFor(in, ic); p.kind == planLowered {
			if m.opts.FaultInjectAt > 0 {
				m.callTicks(f, in, loc)
			}
			var args []*obj
			if m.opts.SecretSan {
				args = m.gather(loc, in.Args)
			}
			loc[in.Dst] = m.postCall(in.Ent, args, m.callFrom(p.f, p.code, loc, in.Args), true)
			return
		}
	}
	m.execArgs(fr, in, ic)
}
