package vm

import (
	"strings"

	"kigumi/internal/mir"
)

func (m *Machine) execArgs(fr *frame, in *mir.Inst, ic *instCode) {
	f := fr.f
	args := m.gather(fr.locals, in.Args)
	var res *obj
	switch in.Op {
	case mir.OpConst:
		res = m.constant(in)
		m.chargeMemory(res)
	case mir.OpGlobalGet:
		res = m.loadGlobal(in.Ent)
	case mir.OpGlobalSet:
		m.storeGlobal(in.Ent, args[0])
		return
	case mir.OpBoxDirect, mir.OpUnboxDirect:
		res = m.cross(fr, in, args[0])
	case mir.OpCall:
		m.callTicks(f, in, fr.locals)
		res = m.callPlanned(in, ic, args)
		if m.p.ResultBoxed(f, *in) {
			m.scalarTick(res)
		}
	case mir.OpCallValue, mir.OpCallC:
		if f.IsDirect(in.Args[0]) {
			closure := m.closureCall(f, in)
			if closure {
				m.argTicks(f, in, args)
			}
			res = m.callDirectValue(args[0], args[1:])
			if closure {
				m.resultTick(res, args[1:])
			}
			break
		}
		m.argTicks(f, in, args)
		res = m.callSite(in, args)
	case mir.OpBuiltin:
		if in.Str == "task.close" {
			m.closeScopeAt(fr)
			res = unitObj
			break
		}
		m.site, m.siteFn = in, f
		res = m.builtin(in.Str, args)
		m.chargeMemory(res)
	case mir.OpFutureNew:
		res = m.newFuture(args)
		m.chargeMemory(res)
	case mir.OpAwait, mir.OpCancelled, mir.OpCancelDone:
		m.abort("`await` outside an async body")
	case mir.OpFutureDone:
		return
	case mir.OpRecord:
		if len(args) > 0 {
			m.aggTicks(f, in, args, 2)
		}
		res = mkRecord(in.Ent, args)
		m.chargeMemory(res)
	case mir.OpVariant:
		if len(args) > 0 {
			m.aggTicks(f, in, args, 2)
		}
		res = mkVariant(in.Ent, args)
		m.chargeMemory(res)
	case mir.OpFieldMove:
		res = m.fieldMove(args[0], in.Index)
	case mir.OpSetField:
		m.storeTick(f, in, in.Args[1], args[1])
		r := deref(m.ownDirect(fr, in.Args[0], args[0]))
		old := r.fields[in.Index]
		r.fields[in.Index] = args[1]
		if !m.borrowField(r, in.Index) {
			m.dropChecked(fr, old)
		}
		return
	case mir.OpArray:
		m.aggTicks(f, in, args, min(2, 1+len(args)))
		res = mkArray(args)
		m.chargeMemory(res)
	case mir.OpSetIndex:
		m.storeTick(f, in, in.Args[2], args[2])
		m.dropChecked(fr, m.setIndex(m.ownDirect(fr, in.Args[0], args[0]), args[1], args[2]))
		return
	case mir.OpIsType:
		v := deref(args[0])
		res = mkBool(v.k == kBox && m.typeEnt(v.dynType()) == in.Ent)
	case mir.OpUnbox:
		v := args[0]
		if v.k == kBox {
			v = retain(v.inner)
		}
		res = v
	case mir.OpBox:
		v := args[0]
		if v.k != kBox {
			b := newObj(kBox)
			b.i, b.inner = int64(in.Type), v
			v = b
			m.chargeMemory(b)
		}
		res = v
	case mir.OpLendBox:
		res = &obj{k: kBox, rc: -1, i: int64(in.Type), inner: deref(args[0]), b: true}
		if args[0].k == kCell {
			res.fields = []*obj{args[0]}
		}
	case mir.OpClosure, mir.OpFnItem, mir.OpBind, mir.OpCFnPtr:
		if len(args) > 0 && (in.Op == mir.OpBind || in.Op == mir.OpClosure && in.Str == "method") {
			m.tick(bindAllocs)
		}
		res = m.closure(in, ic, args)
		m.chargeMemory(res)
	case mir.OpNewCell:
		res = mkCell(args[0])
		res.b = in.Lent
		m.chargeMemory(res)
	case mir.OpCellTake:
		res = args[0].inner
		args[0].inner = nil
	case mir.OpCellSet:
		m.storeTick(f, in, in.Args[1], args[1])
		old := args[0].inner
		args[0].inner = args[1]
		m.dropChecked(fr, old)
		return
	case mir.OpInterp:
		var sb strings.Builder
		ai := 0
		for _, piece := range in.Strs {
			if piece == "" {
				m.displayInto(&sb, args[ai])
				ai++
				continue
			}
			sb.WriteString(piece)
		}
		res = mkStr([]byte(sb.String()))
		m.chargeMemory(res)
	case mir.OpShell:
		res = m.shellPlan(in, args)
	case mir.OpPanic:
		m.abort(in.Str)
	default:
		panic(&Unsupported{What: in.Op.String()})
	}
	fr.locals[in.Dst] = res
}
