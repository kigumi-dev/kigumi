package llgen

import (
	"strings"

	"kigumi/internal/compiler/mir"
	opdef "kigumi/internal/compiler/mir/ops"
)

func (e *emitter) tryDirectInst(sb *strings.Builder, f *mir.Func, in mir.Inst) bool {
	if e.tryFixedInst(sb, f, in) {
		return true
	}
	if in.Op == mir.OpBuiltin && (in.Str == "direct.eq" || in.Str == "direct.hash") {
		e.protoBuiltin(sb, f, in)
		return true
	}
	switch in.Op {
	case mir.OpRecord, mir.OpClosure:
		if a, ok := e.direct[in.Dst]; ok {
			e.buildAgg(sb, a, in)
			return true
		}
	case mir.OpCallValue:
		if a, ok := e.direct[in.Args[0]]; ok && a.Clo != nil {
			e.closureCall(sb, in, a)
			return true
		}
	case mir.OpVariant:
		if a, ok := e.direct[in.Dst]; ok {
			e.buildVariant(sb, a, in)
			return true
		}
	case mir.OpPayload:
		if a, ok := e.direct[in.Args[0]]; ok {
			e.readPayload(sb, f, a, in)
			return true
		}
	case mir.OpIsVariant:
		if a, ok := e.direct[in.Args[0]]; ok {
			e.testVariant(sb, f, a, in)
			return true
		}
	case mir.OpField, mir.OpFieldMove:
		if a, ok := e.direct[in.Args[0]]; ok {
			e.loadLeaf(sb, f, leafOf(a, in), e.fieldPtr(sb, a, aggPtr(in.Args[0]), in.Index), in.Dst)
			return true
		}
	case mir.OpSetField:
		if a, ok := e.direct[in.Args[0]]; ok {
			e.storeLeaf(sb, leafOf(a, in), e.fieldPtr(sb, a, aggPtr(in.Args[0]), in.Index), in.Args[1])
			return true
		}
	case mir.OpAlias, mir.OpMove, mir.OpCopy, mir.OpShare:
		if a, ok := e.direct[in.Dst]; ok && a.counted() {
			e.countedCopyInst(sb, f, a, in)
			return true
		}
		if a, ok := e.direct[in.Dst]; ok {
			copyAgg(sb, a, aggPtr(in.Dst), aggPtr(in.Args[0]))
			return true
		}
	case mir.OpCall:
		if in.DPrim != opdef.DPNone {
			e.directPrimCall(sb, f, in)
			return true
		}
		if fn, ok := e.p.ByEnt[in.Ent]; ok {
			if s := e.directSig(fn); s != nil || e.ssigs[fn] != nil {
				e.directCall(sb, f, in, fn, s)
				return true
			}
		}
	case mir.OpDrop, mir.OpRelease:
		if a, ok := e.countedLocal(in.Args[0]); ok {
			e.refCall(sb, a, refDrop, aggPtr(in.Args[0]))
			return true
		}
		return e.isDirect(in.Args[0])
	case mir.OpFutureDone:
		// The runtime only marks the frame done; the return hands the boxed result over.
		if e.isDirect(in.Args[0]) {
			e.irb(sb).Call(rtFutureComplete, vptr("%fut"), vptr("null"))
			return true
		}
	case mir.OpBoxDirect:
		if key, ok := e.constKey[in.Dst]; ok {
			e.store(sb, in.Dst, e.constArgValue(sb, key))
			return true
		}
		if a, ok := e.agg(in.Type); ok {
			if a.counted() && !f.Locals[in.Args[0]].Cross {
				e.refCall(sb, a, refRetain, aggPtr(in.Args[0]))
			}
			e.store(sb, in.Dst, e.boxAgg(sb, a, aggPtr(in.Args[0])))
			return true
		}
	case mir.OpUnboxDirect:
		if a, ok := e.agg(in.Type); ok {
			p := e.load(sb, in.Args[0])
			e.unboxAgg(sb, a, aggPtr(in.Dst), p)
			if f.Locals[in.Args[0]].Cross {
				e.irb(sb).Call(rtRelease, vptr(p))
			}
			return true
		}
	}
	return false
}

func (e *emitter) buildAgg(sb *strings.Builder, a *aggInfo, in mir.Inst) {
	for i, fld := range a.Fields {
		e.storeLeaf(sb, fld, e.fieldPtr(sb, a, aggPtr(in.Dst), i), in.Args[i])
	}
}

func (e *emitter) instArgs(sb *strings.Builder, in mir.Inst) ([]string, []bool) {
	args := make([]string, 0, len(in.Args))
	boxed := make([]bool, len(in.Args))
	for i, a := range in.Args {
		if in.Op == mir.OpCall {
			e.noteHanded(i)
		}
		if e.isScalar(a) {
			// classifyArgs only lets a scalar-classified local reach here
			// at a non-retaining read; released below, in releaseScalarArgs.
			if in.Op == mir.OpInterp {
				args = append(args, e.scalarFrameBox(sb, a))
				continue
			}
			args = append(args, e.argBoxOnDemand(sb, i, a))
			continue
		}
		args = append(args, e.load(sb, a))
		boxed[i] = e.boxTmp[a]
	}
	return args, boxed
}
