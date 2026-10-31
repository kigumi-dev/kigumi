package llgen

import (
	"strings"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (e *emitter) instBody(sb *strings.Builder, f *midir.Func, in midir.Inst) {
	if e.tryScalarInst(sb, f, in) {
		return
	}
	if e.tryDirectInst(sb, f, in) || e.iterBuiltin(sb, f, in) {
		return
	}
	if in.Op == midir.OpNop {
		return
	}
	args, boxed := e.instArgs(sb, in)
	defer e.releaseAggArgs(sb, in, args, boxed)
	res := ""
	switch in.Op {
	case midir.OpConst:
		res = e.constant(sb, in)
	case midir.OpUnit:
		res = e.irb(sb).Call(rtUnit).Text
	case midir.OpGlobalGet:
		res = e.loadGlobal(sb, in.Ent)
	case midir.OpGlobalSet:
		e.storeGlobal(sb, in.Ent, args[0])
		return
	case midir.OpAlias:
		res = args[0]
	case midir.OpShare:
		res = e.share(sb, f, in, args[0])
	case midir.OpRelease:
		e.dropCall(sb, f, rtRelease, args[0])
		return
	case midir.OpMove:
		res = args[0]
		// A moved-out binding may still be dropped on a path that did not
		// move it; a null slot makes that release a no-op.
		if f.Locals[in.Args[0]].Ent != 0 {
			e.store(sb, in.Args[0], "null")
		}
	case midir.OpCall:
		res = e.callEntity(sb, f, in, args, f.Locals[in.Dst].Type)
	case midir.OpCallValue:
		arr := e.argArray(sb, args[1:])
		if e.isScalar(in.Dst) {
			e.storeNative(sb, in.Dst, e.callValue(sb, in, args, arr, true))
			e.releaseScalarArgs(sb, in, args)
			return
		}
		res = e.callValue(sb, in, args, arr, false).Text
	case midir.OpBuiltin:
		res = e.builtinCall(sb, in, args)
	case midir.OpFutureNew:
		arr := e.argArray(sb, args[1:])
		res = e.irb(sb).Call(rtFutureNew, vptr(args[0]), vi32(len(args)-1), vptr(arr)).Text
	case midir.OpAwait, midir.OpCancelled, midir.OpCancelDone:
		e.asyncOp(sb, in, args)
		return
	case midir.OpFutureDone:
		e.irb(sb).Call(rtFutureComplete, vptr("%fut"), vptr(args[0]))
		return
	case midir.OpBinary:
		res = e.irb(sb).Call(rtBinop, vptr(e.cstr(in.Str)), vptr(args[0]), vptr(args[1])).Text
	case midir.OpUnary:
		if in.Str == "cast" {
			res = e.irb(sb).Call(rtCast, vptr(args[0]), vi32(e.numKind(in.Type))).Text
		} else {
			res = e.irb(sb).Call(rtUnop, vptr(e.cstr(in.Str)), vptr(args[0])).Text
		}
	case midir.OpRecord:
		arr := e.argArray(sb, args)
		res = e.irb(sb).Call(rtRecord, vptr(e.desc(in.Ent)), vi64(len(args)), vptr(arr)).Text
	case midir.OpVariant:
		res = e.variant(sb, in, args)
	case midir.OpField:
		res = e.irb(sb).Call(rtField, vptr(args[0]), vi64(in.Index)).Text
	case midir.OpFieldMove:
		res = e.irb(sb).Call(rtFieldMove, vptr(args[0]), vi64(in.Index)).Text
	case midir.OpSetField:
		e.setField(sb, f, in, args)
		return
	case midir.OpArray:
		arr := e.argArray(sb, args)
		res = e.irb(sb).Call(rtArray, vi64(len(args)), vptr(arr)).Text
	case midir.OpIndex:
		res = e.irb(sb).Call(rtIndex, vptr(args[0]), vptr(args[1]), vptr(e.siteText(f, in))).Text
	case midir.OpSetIndex:
		e.setIndex(sb, f, in, args)
		e.releaseScalarArgs(sb, in, args)
		return
	case midir.OpPayload:
		res = e.irb(sb).Call(rtPayload, vptr(args[0]), vi64(in.Index)).Text
	case midir.OpIsVariant:
		res = e.irb(sb).Call(rtIsvariant, vptr(args[0]), vptr(e.vdesc(in.Ent))).Text
	case midir.OpIsType:
		res = e.irb(sb).Call(rtIstype, vptr(args[0]), vptr(e.desc(in.Ent))).Text
	case midir.OpUnbox:
		res = e.irb(sb).Call(rtUnbox, vptr(args[0])).Text
	case midir.OpBox:
		res = e.irb(sb).Call(rtBox, vptr(args[0]), vptr(e.descOfType(in.Type)), vptr(e.vtable(in.Type, f.Locals[in.Dst].Type, e.r.Entity(f.Ent).Pkg))).Text
	case midir.OpLendBox:
		res = e.lendBox(sb, f, in, args[0])
	case midir.OpAsm:
		res = e.asm(sb, f, in, args)
	case midir.OpCFnPtr:
		res = e.cFnPtr(sb, in.Ent)
	case midir.OpCallC:
		var types []sem.TypeID
		for _, a := range in.Args[1:] {
			types = append(types, f.Locals[a].Type)
		}
		res = e.cPtrCall(sb, args[0], in.Type, args[1:], types)
	case midir.OpBind:
		res = e.bindValue(sb, in, args)
	case midir.OpClosure, midir.OpFnItem:
		if in.Op == midir.OpFnItem && (e.r.Fn(in.Ent).Abi != "" || e.r.Fn(in.Ent).Naked) {
			res = e.cFnPtr(sb, in.Ent)
			break
		}
		if in.Op == midir.OpClosure && (in.Str == "applied" || e.r.Entity(e.r.Entity(in.Ent).Parent).Kind == sem.EntInterface) {
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
			case in.Op == midir.OpClosure && in.Str == "method":
				sym = e.useMethodAdapter(fn)
			default:
				sym = e.useAdapter(fn)
			}
			res = e.markLent(sb, in, len(args), len(args) == 0, e.closureCtor(sb, sym, in.Async, args, arr))
		} else if e.loweredStd(in.Ent) {
			res = e.markLent(sb, in, len(args), len(args) == 0, e.closureCtor(sb, e.stdBound(in.Ent, len(args), f.Locals[in.Dst].Type), in.Async, args, arr))
		} else {
			res = e.irb(sb).Call(rtStdBind, vptr(e.stdKeyStr(e.stdKey(in.Ent))), vi32(mir.LentMask(e.r, in.Ent)>>len(args)), vi32(len(args)), vptr(arr)).Text
		}
	case midir.OpBorrow:
		res = args[0]
	case midir.OpNewCell:
		if in.Scoped && !framed(f, in.Dst) {
			slot := e.stackSlot(sb, objSlotType()+", align 16")
			e.irb(sb).Call(rtCellInit, vptr(slot), vptr(args[0]))
			res = slot
			break
		}
		if in.Lent {
			e.irb(sb).Call(rtRetain, vptr(args[0]))
		}
		res = e.irb(sb).Call(rtCell, vptr(args[0])).Text
	case midir.OpCellGet:
		res = e.irb(sb).Call(rtCellget, vptr(args[0])).Text
	case midir.OpCellTake:
		res = e.irb(sb).Call(rtCellTake, vptr(args[0])).Text
	case midir.OpCellSet:
		e.cellSet(sb, f, args)
		return
	case midir.OpInterp:
		res = e.interp(sb, in, args)
	case midir.OpShell:
		res = e.shell(sb, in, args)
	case midir.OpDrop:
		if e.lentFrameCell(f, in.Args[0]) {
			return
		}
		// Null first so a panicking drop never runs twice on the next scope exit.
		if f.Locals[in.Args[0]].Ent != 0 {
			e.store(sb, in.Args[0], "null")
		}
		e.dropCall(sb, f, rtDrop, args[0])
		return
	case midir.OpPanic:
		e.emitPanic(sb, in)
		return
	default:
		return
	}
	e.releaseScalarArgs(sb, in, args)
	e.storeMaybeScalar(sb, in.Dst, res)
}
