package llgen

import (
	"strings"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

func (e *emitter) instBody(sb *strings.Builder, f *mir.Func, in mir.Inst) {
	if e.tryScalarInst(sb, f, in) {
		return
	}
	if e.tryDirectInst(sb, f, in) {
		return
	}
	if in.Op == mir.OpNop {
		return
	}
	args, boxed := e.instArgs(sb, in)
	defer e.releaseAggArgs(sb, in, args, boxed)
	res := ""
	switch in.Op {
	case mir.OpConst:
		res = e.constant(sb, in)
	case mir.OpUnit:
		res = e.irb(sb).Call(rtUnit).Text
	case mir.OpGlobalGet:
		res = e.loadGlobal(sb, in.Ent)
	case mir.OpGlobalSet:
		e.storeGlobal(sb, in.Ent, args[0])
		return
	case mir.OpAlias:
		res = args[0]
	case mir.OpShare:
		if in.Copy && e.scalarBoxed(f.Locals[in.Args[0]].Type) {
			// rt_copy retains a scalar box, and a retain says so to an IR audit.
			res = e.irb(sb).Call(rtRetain, vptr(args[0])).Text
		} else if in.Copy && in.IfWritable {
			res = e.irb(sb).Call(rtCopyMut, vptr(args[0])).Text
		} else if in.Copy {
			res = e.irb(sb).Call(rtCopy, vptr(args[0])).Text
		} else {
			res = e.irb(sb).Call(rtRetain, vptr(args[0])).Text
		}
	case mir.OpRelease:
		e.dropCall(sb, f, rtRelease, args[0])
		return
	case mir.OpMove:
		res = args[0]
		// A moved-out binding may still be dropped on a path that did not
		// move it; a null slot makes that release a no-op.
		if f.Locals[in.Args[0]].Ent != 0 {
			e.store(sb, in.Args[0], "null")
		}
	case mir.OpCall:
		res = e.callEntity(sb, f, in, args, f.Locals[in.Dst].Type)
	case mir.OpCallValue:
		arr := e.argArray(sb, args[1:])
		res = e.irb(sb).Call(rtCallv, vptr(args[0]), vi32(len(args)-1), vptr(arr)).Text
	case mir.OpBuiltin:
		arr := e.argArray(sb, args)
		res = e.irb(sb).Call(rtBuiltin, vptr(e.cstr(in.Str)), vi32(len(args)), vptr(arr)).Text
	case mir.OpFutureNew:
		arr := e.argArray(sb, args[1:])
		res = e.irb(sb).Call(rtFutureNew, vptr(args[0]), vi32(len(args)-1), vptr(arr)).Text
	case mir.OpAwait:
		e.await(sb, in, args[0])
		return
	case mir.OpFutureDone:
		e.irb(sb).Call(rtFutureComplete, vptr("%fut"), vptr(args[0]))
		return
	case mir.OpBinary:
		res = e.irb(sb).Call(rtBinop, vptr(e.cstr(in.Str)), vptr(args[0]), vptr(args[1])).Text
	case mir.OpUnary:
		if in.Str == "cast" {
			res = e.irb(sb).Call(rtCast, vptr(args[0]), vi32(e.numKind(in.Type))).Text
		} else {
			res = e.irb(sb).Call(rtUnop, vptr(e.cstr(in.Str)), vptr(args[0])).Text
		}
	case mir.OpRecord:
		arr := e.argArray(sb, args)
		res = e.irb(sb).Call(rtRecord, vptr(e.desc(in.Ent)), vi64(len(args)), vptr(arr)).Text
	case mir.OpVariant:
		res = e.variant(sb, in, args)
	case mir.OpField:
		res = e.irb(sb).Call(rtField, vptr(args[0]), vi64(in.Index)).Text
	case mir.OpFieldMove:
		res = e.irb(sb).Call(rtFieldMove, vptr(args[0]), vi64(in.Index)).Text
	case mir.OpSetField:
		e.setField(sb, f, in, args)
		return
	case mir.OpArray:
		arr := e.argArray(sb, args)
		res = e.irb(sb).Call(rtArray, vi64(len(args)), vptr(arr)).Text
	case mir.OpIndex:
		res = e.irb(sb).Call(rtIndex, vptr(args[0]), vptr(args[1])).Text
	case mir.OpSetIndex:
		e.setIndex(sb, f, in, args)
		e.releaseScalarArgs(sb, in, args)
		return
	case mir.OpPayload:
		res = e.irb(sb).Call(rtPayload, vptr(args[0]), vi64(in.Index)).Text
	case mir.OpIsVariant:
		res = e.irb(sb).Call(rtIsvariant, vptr(args[0]), vptr(e.vdesc(in.Ent))).Text
	case mir.OpIsType:
		res = e.irb(sb).Call(rtIstype, vptr(args[0]), vptr(e.desc(in.Ent))).Text
	case mir.OpUnbox:
		res = e.irb(sb).Call(rtUnbox, vptr(args[0])).Text
	case mir.OpBox:
		res = e.irb(sb).Call(rtBox, vptr(args[0]), vptr(e.descOfType(in.Type)), vptr(e.vtable(in.Type, f.Locals[in.Dst].Type, e.r.Entity(f.Ent).Pkg))).Text
	case mir.OpAsm:
		res = e.asm(sb, f, in, args)
	case mir.OpCFnPtr:
		res = e.cFnPtr(sb, in.Ent)
	case mir.OpCallC:
		var types []sem.TypeID
		for _, a := range in.Args[1:] {
			types = append(types, f.Locals[a].Type)
		}
		res = e.cPtrCall(sb, args[0], in.Type, args[1:], types)
	case mir.OpBind:
		fn, own := e.p.ByEnt[in.Ent]
		if !own && len(args) == 0 {
			res = e.stdBindSlot(e.stdKey(in.Ent), mir.LentMask(e.r, in.Ent))
			break
		}
		arr := e.argArray(sb, args)
		if own {
			res = e.closureCtor(sb, e.useBind(fn), false, args, arr)
		} else {
			res = e.irb(sb).Call(rtStdBind, vptr(e.cstr(e.stdKey(in.Ent))), vi32(mir.LentMask(e.r, in.Ent)), vi32(len(args)), vptr(arr)).Text
		}
	case mir.OpClosure, mir.OpFnItem:
		if in.Op == mir.OpFnItem && (e.r.Fn(in.Ent).Abi != "" || e.r.Fn(in.Ent).Naked) {
			res = e.cFnPtr(sb, in.Ent)
			break
		}
		if in.Op == mir.OpClosure && (in.Str == "applied" || e.r.Entity(e.r.Entity(in.Ent).Parent).Kind == sem.EntInterface) {
			res = e.requirementValue(sb, in, args)
			break
		}
		arr := e.argArray(sb, args)
		if fn, ok := e.p.ByEnt[in.Ent]; ok {
			var sym string
			switch {
			// An async body has no adapter: its value carries the frame
			// descriptor a Future is built from.
			case in.Async:
				sym = e.asyncDescSym(fn)
			// A method value carries its receiver in the environment, not in the arguments.
			case in.Op == mir.OpClosure && in.Str == "method":
				sym = e.useMethodAdapter(fn)
			default:
				sym = e.useAdapter(fn)
			}
			res = e.closureCtor(sb, sym, in.Async, args, arr)
		} else if e.loweredStd(in.Ent) {
			res = e.closureCtor(sb, e.stdBound(in.Ent, len(args), f.Locals[in.Dst].Type), in.Async, args, arr)
		} else {
			res = e.irb(sb).Call(rtStdBind, vptr(e.cstr(e.stdKey(in.Ent))), vi32(mir.LentMask(e.r, in.Ent)>>len(args)), vi32(len(args)), vptr(arr)).Text
		}
	case mir.OpBorrow:
		res = args[0]
	case mir.OpNewCell:
		if in.Scoped && !framed(f, in.Dst) {
			slot := e.stackSlot(sb, objSlotType()+", align 16")
			e.irb(sb).Call(rtCellInit, vptr(slot), vptr(args[0]))
			res = slot
			break
		}
		res = e.irb(sb).Call(rtCell, vptr(args[0])).Text
	case mir.OpCellGet:
		res = e.irb(sb).Call(rtCellget, vptr(args[0])).Text
	case mir.OpCellTake:
		res = e.irb(sb).Call(rtCellTake, vptr(args[0])).Text
	case mir.OpCellSet:
		e.cellSet(sb, f, args)
		return
	case mir.OpInterp:
		res = e.interp(sb, in, args)
	case mir.OpShell:
		res = e.shell(sb, in, args)
	case mir.OpDrop:
		// Null first so a panicking drop never runs twice on the next scope exit.
		if f.Locals[in.Args[0]].Ent != 0 {
			e.store(sb, in.Args[0], "null")
		}
		e.dropCall(sb, f, rtDrop, args[0])
		return
	case mir.OpPanic:
		e.irb(sb).Call(rtPanic, vptr(e.cstr(in.Str)))
		return
	default:
		return
	}
	e.releaseScalarArgs(sb, in, args)
	e.storeMaybeScalar(sb, in.Dst, res)
}
