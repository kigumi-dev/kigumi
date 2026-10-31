package llgen

import (
	"fmt"
	"slices"
	"strings"

	"kigumi/internal/compiler/mir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

type unwindFrame struct {
	slot map[mir.LocalID]int
	// agg holds the counted direct locals: slot k is the local's address with bit 0 set, slot k+1 its drop helper.
	agg  map[mir.LocalID]*aggInfo
	n    int
	plan *mir.UnwindPlan
	// inFrame: the slots are the Future's frame words, not an array beside the record.
	inFrame bool
	cur     []uint64
	known   bool
}

func (e *emitter) wantsCatch(opts EmitOptions) bool {
	if !opts.Catch {
		return false
	}
	if opts.Reachable == nil {
		return true
	}
	return slices.Contains(opdef.CapabilityMacros(opts.Reachable.Ops), "KIGUMI_CAP_TASK_EXECUTOR") || e.p.ReachesDropHook(opts.Reachable)
}

// Exports and section-placed bodies are entered from outside the Kigumi call tree.
func (e *emitter) unwindFor(f *mir.Func) *unwindFrame {
	if !e.catch || f.Async {
		return nil
	}
	if ent := e.r.Entity(f.Ent); ent.Kind == sem.EntFn && (e.r.Fn(f.Ent).Export != "" || e.r.FnSectionOf(f.Ent) != "") {
		return nil
	}
	plan := e.p.UnwindPlanOf(f)
	slot := map[mir.LocalID]int{}
	agg := map[mir.LocalID]*aggInfo{}
	var ids []mir.LocalID
	for _, set := range plan.Sets[1:] {
		for _, l := range set {
			a, direct := e.direct[l]
			if direct && !a.counted() {
				continue
			}
			if _, ok := e.scalar[l]; ok {
				continue
			}
			if _, seen := slot[l]; !seen {
				slot[l] = 0
				ids = append(ids, l)
			}
			if direct {
				agg[l] = a
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	slices.Sort(ids)
	n := 0
	for _, l := range ids {
		slot[l] = n
		n++
		if agg[l] != nil {
			n++
		}
	}
	return &unwindFrame{slot: slot, agg: agg, n: n, plan: plan}
}

func (u *unwindFrame) words() int { return (u.n + 63) / 64 }

// Must match kg_uframe in rt_core.c: a three-word header, the mask words, then the slots.
const unwindHeaderWords = 3

func (u *unwindFrame) slotWord(k int) int { return unwindHeaderWords + u.words() + k }

func (e *emitter) unwindAllocas(sb *strings.Builder) {
	u := e.unwind
	if u == nil {
		return
	}
	if u.inFrame {
		e.asyncUnwindAllocas(sb)
		return
	}
	fmt.Fprintf(sb, "  %%urec = alloca [%d x i64], align 8\n", unwindHeaderWords+u.words()+u.n)
	fmt.Fprintf(sb, "  %%umask = getelementptr i64, ptr %%urec, i64 %d\n", unwindHeaderWords)
	fmt.Fprintf(sb, "  call void @llvm.memset.p0.i64(ptr %%umask, i8 0, i64 %d, i1 false)\n", 8*u.words())
}

func (e *emitter) slotAddr(sb *strings.Builder, l mir.LocalID) bool {
	if e.unwind == nil {
		return false
	}
	k, ok := e.unwind.slot[l]
	if !ok || e.unwind.agg[l] != nil {
		return false
	}
	fmt.Fprintf(sb, "  %%l%d = getelementptr i64, ptr %%urec, i64 %d\n", l, e.unwind.slotWord(k))
	return true
}

// A box the call site is about to release stops being a sweep's to release, in case its drop hook panics.
func (e *emitter) unslot(sb *strings.Builder, l mir.LocalID) {
	if e.unwind == nil {
		return
	}
	if _, ok := e.unwind.slot[l]; ok && e.unwind.agg[l] == nil {
		e.store(sb, l, "null")
	}
}

func (e *emitter) unwindEnter(sb *strings.Builder) {
	e.unwindAggSlots(sb)
	switch {
	case e.unwind == nil:
	case e.unwind.inFrame:
		e.irb(sb).Call(rtUnwindEnterSlots, vptr("%urec"), vptr("%slots"), vptr("%umask"), vi64(e.unwind.n))
	default:
		e.irb(sb).Call(rtUnwindEnter, vptr("%urec"), vi64(e.unwind.n))
	}
}

func (e *emitter) unwindLeave(sb *strings.Builder) {
	if e.unwind != nil {
		e.irb(sb).Call(rtUnwindLeave, vptr("%urec"))
	}
}

func (e *emitter) unwindBlock() {
	if e.unwind != nil {
		e.unwind.known = false
	}
}

func (e *emitter) unwindSite(sb *strings.Builder, bid mir.BlockID, pc int) {
	u := e.unwind
	if u == nil || int(bid) >= len(u.plan.At) || u.plan.At[bid][pc] < 0 {
		return
	}
	e.storeMask(sb, bid, pc)
}

func (e *emitter) storeMask(sb *strings.Builder, bid mir.BlockID, pc int) {
	u := e.unwind
	want := make([]uint64, u.words())
	for _, l := range u.plan.SetAt(bid, pc) {
		if k, ok := u.slot[l]; ok {
			want[k/64] |= 1 << (k % 64)
		}
	}
	for w, bits := range want {
		if u.known && u.cur[w] == bits {
			continue
		}
		p := e.newTmp()
		fmt.Fprintf(sb, "  %s = getelementptr i64, ptr %%umask, i64 %d\n", p, w)
		fmt.Fprintf(sb, "  store i64 %d, ptr %s\n", int64(bits), p)
	}
	u.cur, u.known = want, true
}
