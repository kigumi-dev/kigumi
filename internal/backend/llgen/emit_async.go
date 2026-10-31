package llgen

import (
	"fmt"
	"strconv"
	"strings"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
)

func (e *emitter) asyncDescSym(f *mir.Func) string { return "@\"" + e.symName(f) + "$async\"" }
func (e *emitter) frameDropSym(f *mir.Func) string { return "@\"" + e.symName(f) + "$framedrop\"" }

// asyncFunction emits an async body as a poll function over its Future's
// frame: the entry switch resumes where it stopped, a null return is Pending.
func (e *emitter) asyncFunction(f *mir.Func) string {
	var sb strings.Builder
	e.tmp = 0
	e.classifyBody(f)
	e.sig, e.ssig = nil, nil
	e.unwind, e.outer = e.asyncUnwindFor(f), nil
	defer func() { e.unwind = nil }()
	fmt.Fprintf(&sb, "define ptr %s(ptr %%fut) {\nentry:\n", e.sym(f))
	sb.WriteString("  %slots = call ptr @rt_future_slots(ptr %fut)\n")
	e.unwindAllocas(&sb)
	for i := range f.Locals {
		l := mir.LocalID(i)
		if slot := f.Slot[i]; slot >= 0 {
			e.checkSlot(f, l)
			fmt.Fprintf(&sb, "  %%l%d = getelementptr ptr, ptr %%slots, i64 %d\n", i, slot)
			continue
		}
		e.frameLocal(&sb, l)
	}
	var body strings.Builder
	e.entryAllocas, e.hoisting = nil, true
	stores := e.p.CopyStores(f)
	for bid, blk := range f.Blocks {
		fmt.Fprintf(&body, "b%d:\n", bid)
		e.unwindBlock()
		for pc, in := range copyStoreInsts(&blk, bid, stores) {
			e.at = siteAt{mir.BlockID(bid), pc}
			e.unwindSite(&body, e.at.blk, pc)
			e.inst(&body, f, in)
		}
		if endsCancelled(blk) {
			e.cancelExit(&body)
			continue
		}
		e.term(&body, blk.Term)
	}
	e.hoisting = false
	for _, a := range e.entryAllocas {
		sb.WriteString(a)
	}
	e.unwindEnter(&sb)
	sb.WriteString("  %st = call i32 @rt_future_state(ptr %fut)\n")
	sb.WriteString("  switch i32 %st, label %resume.bad [ i32 0, label %start")
	for k := 1; k <= f.Suspends; k++ {
		fmt.Fprintf(&sb, " i32 %d, label %%rs%d", k, k)
	}
	sb.WriteString(" ]\nresume.bad:\n")
	e.irb(&sb).Call(rtPanic, vptr(e.cstr("this async body cannot resume from here")))
	sb.WriteString("  unreachable\nstart:\n")
	for i, prm := range f.Params {
		t := e.newTmp()
		fmt.Fprintf(&sb, "  %s = call ptr @rt_future_arg(ptr %%fut, i64 %d)\n", t, i)
		fmt.Fprintf(&sb, "  store ptr %s, ptr %%l%d\n", t, prm)
	}
	sb.WriteString("  call void @rt_future_started(ptr %fut)\n")
	sb.WriteString("  br label %b0\n")
	sb.WriteString(body.String())
	sb.WriteString("}\n\n")
	sb.WriteString(e.frameDrop(f))
	return sb.String()
}

// await is one suspension point: the operand moves into the frame and the
// resume label re-polls it, returning Pending or Cancelled to the executor.
func (e *emitter) await(sb *strings.Builder, in mir.Inst, operand string) {
	k := in.Index
	e.irb(sb).Call(rtFutureSuspend, vptr("%fut"), vi32(k), vptr(operand))
	fmt.Fprintf(sb, "  br label %%rs%d\nrs%d:\n", k, k)
	e.unwindResume(sb)
	resume := rtFutureResume
	if in.Str == "settle" {
		resume = rtFutureSettle
	}
	r := e.irb(sb).Call(resume, vptr("%fut")).Text
	p := e.newTmp()
	fmt.Fprintf(sb, "  %s = icmp eq ptr %s, null\n", p, r)
	fmt.Fprintf(sb, "  br i1 %s, label %%pend%d, label %%ck%d\n", p, k, k)
	fmt.Fprintf(sb, "pend%d:\n", k)
	e.unwindLeave(sb)
	fmt.Fprintf(sb, "  ret ptr null\nck%d:\n", k)
	c := e.newTmp()
	fmt.Fprintf(sb, "  %s = icmp eq ptr %s, @kg_cancel_marker\n", c, r)
	fmt.Fprintf(sb, "  br i1 %s, label %%can%d, label %%cont%d\n", c, k, k)
	fmt.Fprintf(sb, "can%d:\n  store ptr null, ptr %%l%d\n  br label %%join%d\n", k, in.Dst, k)
	fmt.Fprintf(sb, "cont%d:\n", k)
	e.store(sb, in.Dst, r)
	fmt.Fprintf(sb, "  br label %%join%d\njoin%d:\n", k, k)
	e.cancelFlag = fmt.Sprintf("%%cf%d", k)
	fmt.Fprintf(sb, "  %s = phi i1 [ true, %%can%d ], [ false, %%cont%d ]\n", e.cancelFlag, k, k)
}

func (e *emitter) asyncOp(sb *strings.Builder, in mir.Inst, args []string) {
	switch in.Op {
	case mir.OpAwait:
		e.await(sb, in, args[0])
	case mir.OpCancelled:
		e.store(sb, in.Dst, e.boolBox(sb, e.cancelFlag))
	}
}

func (e *emitter) cancelExit(sb *strings.Builder) {
	e.irb(sb).Call(rtFutureComplete, vptr("%fut"), vptr("null"))
	e.unwindLeave(sb)
	sb.WriteString("  ret ptr @kg_cancel_marker\n")
}

// framePanicked must match KG_FRAME_PANICKED in rt_core.c.
const framePanicked = 1 << 30

func (e *emitter) frameDrop(f *mir.Func) string {
	var sb strings.Builder
	e.tmp = 0
	fmt.Fprintf(&sb, "define internal void %s(ptr %%fut) {\nentry:\n", e.frameDropSym(f))
	sb.WriteString("  %slots = call ptr @rt_future_slots(ptr %fut)\n")
	sb.WriteString("  %st = call i32 @rt_future_state(ptr %fut)\n")
	sb.WriteString("  switch i32 %st, label %done [")
	for k := 1; k <= f.Suspends && k < len(f.Cleanup); k++ {
		if len(e.carrierSlots(f.Cleanup[k])) > 0 {
			fmt.Fprintf(&sb, " i32 %d, label %%d%d", k, k)
		}
	}
	panicDrop := len(e.carrierSlots(f.PanicDrop)) > 0
	if panicDrop {
		fmt.Fprintf(&sb, " i32 %d, label %%dp", framePanicked)
	}
	sb.WriteString(" ]\n")
	for k := 1; k <= f.Suspends && k < len(f.Cleanup); k++ {
		if len(e.carrierSlots(f.Cleanup[k])) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "d%d:\n", k)
		e.releaseSlots(&sb, f, f.Cleanup[k])
	}
	if panicDrop {
		sb.WriteString("dp:\n")
		e.releaseSlots(&sb, f, f.PanicDrop)
	}
	sb.WriteString("done:\n  ret void\n}\n\n")
	return sb.String()
}

func (e *emitter) releaseSlots(sb *strings.Builder, f *mir.Func, locals []mir.LocalID) {
	for _, l := range e.carrierSlots(locals) {
		p, v := e.newTmp(), e.newTmp()
		fmt.Fprintf(sb, "  %s = getelementptr ptr, ptr %%slots, i64 %d\n", p, f.Slot[l])
		fmt.Fprintf(sb, "  %s = load ptr, ptr %s\n", v, p)
		fmt.Fprintf(sb, "  store ptr null, ptr %s\n", p)
		e.irb(sb).Call(rtRelease, vptr(v))
	}
	sb.WriteString("  br label %done\n")
}

// asyncDesc is what a Future is built from: the poll and drop functions, the frame's slot count, whether a pool may
// move it between workers, and which parameters take a `&mut` write-back cell's referent (mir.ReferentParams).
func (e *emitter) asyncDesc(f *mir.Func) string {
	mobile := 0
	if e.r.Entity(f.Ent).Kind == sem.EntFn && e.r.Fn(f.Ent).Mobility == sem.MobilityMigratable {
		mobile = 1
	}
	refs := e.p.ReferentParams(f)
	flags := make([]string, len(refs))
	for i, r := range refs {
		flags[i] = "i8 " + strconv.Itoa(boolBit(r))
	}
	init := "zeroinitializer"
	if len(flags) > 0 {
		init = "[" + strings.Join(flags, ", ") + "]"
	}
	sym := e.asyncDescSym(f)
	refSym := strings.TrimSuffix(sym, "\"") + "$referent\""
	return fmt.Sprintf("%s = private constant [%d x i8] %s\n%s = internal global { ptr, ptr, i32, i32, i32, ptr } { ptr %s, ptr %s, i32 %d, i32 %d, i32 %d, ptr %s }, align 8\n",
		refSym, len(refs), init, sym, e.sym(f), e.frameDropSym(f), f.Frame, mobile, len(refs), refSym)
}

func endsCancelled(blk mir.Block) bool {
	n := len(blk.Insts)
	return n > 0 && blk.Insts[n-1].Op == mir.OpCancelDone
}
