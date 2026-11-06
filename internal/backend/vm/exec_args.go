package vm

import (
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
)

func (m *Machine) execArgs(fr *frame, in *midir.Inst, ic *instCode) {
	f := fr.f
	args := m.gather(fr.locals, in.Args)
	var res *obj
	switch in.Op {
	case midir.OpConst:
		res = m.constant(in)
		m.chargeMemory(res)
	case midir.OpGlobalGet:
		res = m.loadGlobal(in.Ent)
	case midir.OpGlobalSet:
		m.storeGlobal(in.Ent, args[0])
		return
	case midir.OpBoxDirect, midir.OpUnboxDirect:
		res = m.cross(fr, in, args[0])
	case midir.OpCall:
		m.callTicks(f, in, fr.locals)
		m.tryBare = in.DPrim == opdef.DPTryRef || in.DPrim == opdef.DPTryOpt
		res = m.callPlanned(in, ic, args)
		m.tryBare = false
		if m.p.ResultBoxed(f, *in) {
			m.scalarTick(res)
		}
		if m.opts.SecretSan {
			bodyRan := m.planFor(in, ic).kind == planLowered
			if !bodyRan {
				_, bodyRan = m.lowered(in.Ent)
			}
			res = m.postCall(in.Ent, args, res, bodyRan)
		}
	case midir.OpCallValue, midir.OpCallC:
		if f.IsDirect(in.Args[0]) {
			closure := m.closureCall(f, in)
			if closure {
				m.argTicks(f, in, args)
			}
			res = m.callDirectValue(args[0], args[1:])
			if closure {
				m.resultTick(res, args[1:])
			}
			if m.opts.SecretSan {
				res = m.postCallValue(in.Ent, res)
			}
			break
		}
		m.argTicks(f, in, args)
		res = m.callSite(in, args)
		if m.opts.SecretSan {
			res = m.postCallValue(in.Ent, res)
		}
	case midir.OpBuiltin:
		if in.Str == "task.close" {
			m.closeScopeAt(fr)
			res = unitObj
			break
		}
		m.site, m.siteFn = in, f
		if m.opts.SecretSan && (in.Str == "print" || in.Str == "eprint" || in.Str == "panic") {
			m.checkHostIO(args, in.Str)
		}
		res = m.builtin(in.Str, args)
		m.chargeMemory(res)
	case midir.OpFutureNew:
		res = m.newFuture(args)
		m.chargeMemory(res)
	case midir.OpAwait, midir.OpCancelled, midir.OpCancelDone:
		m.abort("`await` outside an async body")
	case midir.OpFutureDone:
		return
	case midir.OpRecord:
		m.aggTicks(f, in, args, 2)
		res = mkRecord(in.Ent, args)
		m.chargeMemory(res)
		if m.opts.SecretSan {
			res = m.withTaint(res, m.joinTaint(args))
		}
	case midir.OpVariant:
		if len(args) > 0 {
			m.aggTicks(f, in, args, 2)
		}
		res = mkVariant(in.Ent, args)
		m.chargeMemory(res)
		if m.opts.SecretSan {
			res = m.withTaint(res, m.joinTaint(args))
		}
	case midir.OpFieldMove:
		res = m.fieldMove(args[0], in.Index)
	case midir.OpSetField:
		m.storeTick(f, in, in.Args[1], args[1])
		r := deref(m.ownDirect(fr, in.Args[0], args[0]))
		old := r.fields[in.Index]
		r.fields[in.Index] = args[1]
		if m.opts.SecretSan {
			m.relabelContainer(r)
		}
		if !m.borrowField(r, in.Index) {
			m.dropChecked(fr, old)
		}
		return
	case midir.OpArray:
		m.aggTicks(f, in, args, min(2, 1+len(args)))
		res = mkArray(args)
		m.chargeMemory(res)
		if m.opts.SecretSan {
			res = m.withTaint(res, m.joinTaint(args))
		}
	case midir.OpSetIndex:
		m.storeTick(f, in, in.Args[2], args[2])
		m.dropChecked(fr, m.setIndex(m.ownDirect(fr, in.Args[0], args[0]), args[1], args[2]))
		return
	case midir.OpIsType:
		v := deref(args[0])
		res = mkBool(v.k == kBox && m.typeEnt(v.dynType()) == in.Ent)
	case midir.OpUnbox:
		v := args[0]
		if v.k == kBox {
			v = m.deriveTaint(retain(v.inner), m.taintOf(v))
		}
		res = v
	case midir.OpBox:
		v := args[0]
		if v.k != kBox {
			b := newObj(kBox)
			b.i, b.inner = int64(in.Type), v
			m.chargeMemory(b)
			v = m.withTaint(b, m.taintOf(args[0]))
		}
		res = v
	case midir.OpLendBox:
		res = &obj{k: kBox, rc: -1, i: int64(in.Type), inner: deref(args[0]), b: true}
		if args[0].k == kCell {
			res.fields = []*obj{args[0]}
		}
	case midir.OpClosure, midir.OpFnItem, midir.OpBind, midir.OpCFnPtr:
		if len(args) > 0 && (in.Op == midir.OpBind || in.Op == midir.OpClosure && in.Str == "method") {
			m.tick(bindAllocs)
		}
		res = m.closure(in, ic, args)
		m.chargeMemory(res)
	case midir.OpNewCell:
		res = mkCell(args[0])
		res.b = in.Lent
		m.chargeMemory(res)
	case midir.OpCellTake:
		res = args[0].inner
		args[0].inner = nil
	case midir.OpCellSet:
		m.storeTick(f, in, in.Args[1], args[1])
		old := args[0].inner
		args[0].inner = args[1]
		m.dropChecked(fr, old)
		return
	case midir.OpInterp:
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
		m.tick(1)
		res = mkStr([]byte(sb.String()))
		m.chargeMemory(res)
		if m.opts.SecretSan {
			res = m.withTaint(res, m.joinTaint(args))
		}
	case midir.OpShell:
		if m.opts.SecretSan {
			m.checkHostIO(args, "shell")
		}
		res = m.shellPlan(in, args)
	case midir.OpPanic:
		m.abort(in.Str)
	default:
		panic(&Unsupported{What: in.Op.String()})
	}
	fr.locals[in.Dst] = res
}
