package llgen

import (
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
)

func (e *emitter) setField(sb *strings.Builder, f *midir.Func, in midir.Inst, args []string) {
	fn, swap := rtSetfield, rtSetfieldSwap
	if in.Copy {
		fn, swap = rtSetfieldCopy, rtSetfieldCopySwap
	}
	if e.outer != nil {
		e.dropCall(sb, f, rtRelease, e.irb(sb).Call(swap, vptr(args[0]), vi64(in.Index), vptr(args[1])).Text)
		return
	}
	e.irb(sb).Call(fn, vptr(args[0]), vi64(in.Index), vptr(args[1]))
}

func (e *emitter) setIndex(sb *strings.Builder, f *midir.Func, in midir.Inst, args []string) {
	fn, swap := rtSetindex, rtSetindexSwap
	if in.Copy {
		fn, swap = rtSetindexCopy, rtSetindexCopySwap
	}
	if e.outer != nil {
		e.dropCall(sb, f, rtRelease, e.irb(sb).Call(swap, vptr(args[0]), vptr(args[1]), vptr(args[2]), vptr(e.siteText(f, in))).Text)
		return
	}
	e.irb(sb).Call(fn, vptr(args[0]), vptr(args[1]), vptr(args[2]), vptr(e.siteText(f, in)))
}

func (e *emitter) cellSet(sb *strings.Builder, f *midir.Func, args []string) {
	if e.outer != nil {
		e.dropCall(sb, f, rtRelease, e.irb(sb).Call(rtCellsetSwap, vptr(args[0]), vptr(args[1])).Text)
		return
	}
	e.irb(sb).Call(rtCellset, vptr(args[0]), vptr(args[1]))
}
