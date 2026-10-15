package llgen

import (
	"strings"

	"kigumi/internal/mir"
)

func (e *emitter) setField(sb *strings.Builder, f *mir.Func, in mir.Inst, args []string) {
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

func (e *emitter) setIndex(sb *strings.Builder, f *mir.Func, in mir.Inst, args []string) {
	fn, swap := rtSetindex, rtSetindexSwap
	if in.Copy {
		fn, swap = rtSetindexCopy, rtSetindexCopySwap
	}
	if e.outer != nil {
		e.dropCall(sb, f, rtRelease, e.irb(sb).Call(swap, vptr(args[0]), vptr(args[1]), vptr(args[2])).Text)
		return
	}
	e.irb(sb).Call(fn, vptr(args[0]), vptr(args[1]), vptr(args[2]))
}

func (e *emitter) cellSet(sb *strings.Builder, f *mir.Func, args []string) {
	if e.outer != nil {
		e.dropCall(sb, f, rtRelease, e.irb(sb).Call(rtCellsetSwap, vptr(args[0]), vptr(args[1])).Text)
		return
	}
	e.irb(sb).Call(rtCellset, vptr(args[0]), vptr(args[1]))
}
