package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

func (e *emitter) asyncDescSym(f *mir.Func) string { return "@\"" + e.symName(f) + "$async\"" }
func (e *emitter) frameDropSym(f *mir.Func) string { return "@\"" + e.symName(f) + "$framedrop\"" }

// asyncFunction emits an async body as a poll function over its Future's
// frame: the entry switch resumes where it stopped, a null return is Pending.
func (e *emitter) asyncFunction(f *mir.Func) string {
	var sb strings.Builder
	e.tmp = 0
	e.scalar = map[mir.LocalID]scalarInfo{}
	e.direct = e.classifyDirect(f)
	e.boxTmp = map[mir.LocalID]bool{}
	e.sig = nil
	fmt.Fprintf(&sb, "define ptr %s(ptr %%fut) {\nentry:\n", e.sym(f))
	sb.WriteString("  %slots = call ptr @rt_future_slots(ptr %fut)\n")
	for i := range f.Locals {
		if slot := f.Slot[i]; slot >= 0 {
			fmt.Fprintf(&sb, "  %%l%d = getelementptr ptr, ptr %%slots, i64 %d\n", i, slot)
			continue
		}
		// Only a crossing's own local is unboxed here: every value that
		// lives across a suspension is one pointer in a frame slot.
		if a, ok := e.direct[mir.LocalID(i)]; ok {
			fmt.Fprintf(&sb, "  %%l%d = alloca %s\n", i, a.Name)
			fmt.Fprintf(&sb, "  store %s zeroinitializer, ptr %%l%d\n", a.Name, i)
			continue
		}
		fmt.Fprintf(&sb, "  %%l%d = alloca ptr\n", i)
		fmt.Fprintf(&sb, "  store ptr null, ptr %%l%d\n", i)
	}
	var body strings.Builder
	e.entryAllocas, e.hoisting = nil, true
	for bid, blk := range f.Blocks {
		fmt.Fprintf(&body, "b%d:\n", bid)
		for _, in := range blk.Insts {
			e.inst(&body, f, in)
		}
		e.term(&body, blk.Term)
	}
	e.hoisting = false
	for _, a := range e.entryAllocas {
		sb.WriteString(a)
	}
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
	r := e.irb(sb).Call(rtFutureResume, vptr("%fut")).Text
	p := e.newTmp()
	fmt.Fprintf(sb, "  %s = icmp eq ptr %s, null\n", p, r)
	fmt.Fprintf(sb, "  br i1 %s, label %%pend%d, label %%ck%d\n", p, k, k)
	fmt.Fprintf(sb, "pend%d:\n  ret ptr null\nck%d:\n", k, k)
	c := e.newTmp()
	fmt.Fprintf(sb, "  %s = icmp eq ptr %s, @kg_cancel_marker\n", c, r)
	fmt.Fprintf(sb, "  br i1 %s, label %%can%d, label %%cont%d\n", c, k, k)
	fmt.Fprintf(sb, "can%d:\n  ret ptr @kg_cancel_marker\ncont%d:\n", k, k)
	e.store(sb, in.Dst, r)
}

// frameDrop releases what a frame abandoned at a suspension point still
// owns, in the order the ownership plan would have released it.
func (e *emitter) frameDrop(f *mir.Func) string {
	var sb strings.Builder
	e.tmp = 0
	fmt.Fprintf(&sb, "define internal void %s(ptr %%fut) {\nentry:\n", e.frameDropSym(f))
	sb.WriteString("  %slots = call ptr @rt_future_slots(ptr %fut)\n")
	sb.WriteString("  %st = call i32 @rt_future_state(ptr %fut)\n")
	sb.WriteString("  switch i32 %st, label %done [")
	for k := 1; k <= f.Suspends && k < len(f.Cleanup); k++ {
		if len(f.Cleanup[k]) > 0 {
			fmt.Fprintf(&sb, " i32 %d, label %%d%d", k, k)
		}
	}
	sb.WriteString(" ]\n")
	for k := 1; k <= f.Suspends && k < len(f.Cleanup); k++ {
		if len(f.Cleanup[k]) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "d%d:\n", k)
		for _, l := range f.Cleanup[k] {
			p, v := e.newTmp(), e.newTmp()
			fmt.Fprintf(&sb, "  %s = getelementptr ptr, ptr %%slots, i64 %d\n", p, f.Slot[l])
			fmt.Fprintf(&sb, "  %s = load ptr, ptr %s\n", v, p)
			fmt.Fprintf(&sb, "  store ptr null, ptr %s\n", p)
			e.irb(&sb).Call(rtRelease, vptr(v))
		}
		sb.WriteString("  br label %done\n")
	}
	sb.WriteString("done:\n  ret void\n}\n\n")
	return sb.String()
}

// asyncDesc is what a Future is built from: the poll and drop functions, the
// frame's slot count, and whether a pool may move it between workers.
func (e *emitter) asyncDesc(f *mir.Func) string {
	mobile := 0
	if e.r.Entity(f.Ent).Kind == sem.EntFn && e.r.Fn(f.Ent).Mobility == sem.MobilityMigratable {
		mobile = 1
	}
	return fmt.Sprintf("%s = internal global { ptr, ptr, i32, i32 } { ptr %s, ptr %s, i32 %d, i32 %d }, align 8\n",
		e.asyncDescSym(f), e.sym(f), e.frameDropSym(f), f.Frame, mobile)
}
