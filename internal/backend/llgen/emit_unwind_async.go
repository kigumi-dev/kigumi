package llgen

import (
	"fmt"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
)

func (e *emitter) asyncUnwindFor(f *midir.Func) *unwindFrame {
	if !e.catch {
		return nil
	}
	plan := e.p.UnwindPlanOf(f)
	slot := map[midir.LocalID]int{}
	for _, set := range plan.Sets[1:] {
		for _, l := range e.sweepLocals(set) {
			if f.Slot[l] >= 0 {
				slot[l] = f.Slot[l]
			}
		}
	}
	if len(slot) == 0 {
		return nil
	}
	return &unwindFrame{slot: slot, n: f.Frame, plan: plan, inFrame: true}
}

func (e *emitter) asyncUnwindAllocas(sb *strings.Builder) {
	u := e.unwind
	sb.WriteString("  %urec = alloca [6 x i64], align 8\n")
	fmt.Fprintf(sb, "  %%umask = alloca [%d x i64]\n", u.words())
	fmt.Fprintf(sb, "  call void @llvm.memset.p0.i64(ptr %%umask, i8 0, i64 %d, i1 false)\n", 8*u.words())
}

// A resume enters with a clear mask, and the awaited child can fault before the next site writes one.
func (e *emitter) unwindResume(sb *strings.Builder) {
	if e.unwind == nil || e.at.blk >= midir.BlockID(len(e.unwind.plan.At)) {
		return
	}
	e.unwind.known = false
	e.storeMask(sb, e.at.blk, e.at.pc)
}
