package llgen

import (
	"fmt"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
)

func (a *aggInfo) counted() bool {
	for _, v := range a.Variants {
		if refSlots(v) != nil {
			return true
		}
	}
	return false
}

func (f aggField) counts() bool { return f.Ref || f.Agg != nil && f.Agg.counted() }

func (e *emitter) countedLocal(l midir.LocalID) (*aggInfo, bool) {
	a, ok := e.direct[l]
	return a, ok && a.counted()
}

func (e *emitter) storeRef(sb *strings.Builder, p, v string) {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = ptrtoint ptr %s to i64\n  store i64 %s, ptr %s\n", t, v, t, p)
}

func (e *emitter) loadRef(sb *strings.Builder, p string) string {
	x, t := e.newTmp(), e.newTmp()
	fmt.Fprintf(sb, "  %s = load i64, ptr %s\n  %s = inttoptr i64 %s to ptr\n", x, p, t, x)
	return t
}

// Each ref op applies one runtime call to the references the live variant holds; drop zeroes the value before it releases.
const (
	refRetain  = "retain"
	refCopy    = "copy"
	refCopyMut = "copymut"
	refDrop    = "drop"
)

func (e *emitter) refSym(a *aggInfo, op string) string {
	return "@\"kg." + op + "." + a.Label + "\""
}

func (e *emitter) refCall(sb *strings.Builder, a *aggInfo, op, base string) {
	fmt.Fprintf(sb, "  call void %s(ptr %s)\n", e.refHelper(a, op), base)
}

func (e *emitter) refHelper(a *aggInfo, op string) string {
	key := a.Label + "\x00" + op
	if !e.refDone[key] {
		e.refDone[key] = true
		e.helpers.WriteString(e.refFn(a, op))
	}
	return e.refSym(a, op)
}

func (e *emitter) refFn(a *aggInfo, op string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "define internal void %s(ptr %%v) {\nentry:\n", e.refSym(a, op))
	e.adtPrelude(&sb, a, "%v")
	fmt.Fprintf(&sb, "  %%tag = load %s, ptr %%tp\n", adtTag)
	fmt.Fprintf(&sb, "  switch %s %%tag, label %%done [", adtTag)
	for i, v := range a.Variants {
		if refSlots(v) != nil {
			fmt.Fprintf(&sb, " %s %d, label %%c%d", adtTag, i, i)
		}
	}
	sb.WriteString(" ]\n")
	for i, v := range a.Variants {
		slots := refSlots(v)
		if slots == nil {
			continue
		}
		fmt.Fprintf(&sb, "c%d:\n", i)
		var ptrs, at []string
		for _, j := range slots {
			g := e.gepSlot(&sb, j, a)
			if sub := v.Payload[j].Agg; sub != nil {
				e.refCall(&sb, sub, op, g)
				continue
			}
			ptrs, at = append(ptrs, e.loadRef(&sb, g)), append(at, g)
		}
		if op == refDrop {
			zeroAgg(&sb, a, "%v")
		}
		for k, p := range ptrs {
			switch op {
			case refRetain:
				fmt.Fprintf(&sb, "  call ptr @rt_retain(ptr %s)\n", p)
			case refDrop:
				fmt.Fprintf(&sb, "  call void @rt_release(ptr %s)\n", p)
			default:
				c := e.newTmp()
				fmt.Fprintf(&sb, "  %s = call ptr @%s(ptr %s)\n", c, map[string]string{refCopy: "rt_copy", refCopyMut: "rt_copy_mut"}[op], p)
				e.storeRef(&sb, at[k], c)
			}
		}
		sb.WriteString("  br label %done\n")
	}
	sb.WriteString("done:\n  ret void\n}\n\n")
	return sb.String()
}

func refSlots(v aggVariant) []int {
	var out []int
	for j, f := range v.Payload {
		if f.counts() {
			out = append(out, j)
		}
	}
	return out
}

// shareRefs gives a copied value its own counts the way the boxed share would: rt_copy, rt_copy_mut or rt_retain per reference.
func (e *emitter) shareRefs(sb *strings.Builder, a *aggInfo, in midir.Inst) {
	op := refRetain
	switch {
	case (in.Op == midir.OpShare || in.Op == midir.OpCopy) && in.Copy && in.IfWritable:
		op = refCopyMut
	case (in.Op == midir.OpShare || in.Op == midir.OpCopy) && in.Copy:
		op = refCopy
	}
	e.refCall(sb, a, op, aggPtr(in.Dst))
}

func (e *emitter) countedCopyInst(sb *strings.Builder, f *midir.Func, a *aggInfo, in midir.Inst) {
	copyAgg(sb, a, aggPtr(in.Dst), aggPtr(in.Args[0]))
	switch {
	case in.Op == midir.OpShare || in.Op == midir.OpCopy:
		e.shareRefs(sb, a, in)
	case in.Op == midir.OpMove && f.Locals[in.Args[0]].Ent != 0:
		zeroAgg(sb, a, aggPtr(in.Args[0]))
	}
}

type countedCross struct {
	src                 midir.LocalID
	agg                 *aggInfo
	unbox, hands, named bool
}

// countedCrosses names the crossing temporaries of a counted value; each holds counts of its own until its one reader settles them.
func (e *emitter) countedCrosses(f *midir.Func) map[midir.LocalID]countedCross {
	out := map[midir.LocalID]countedCross{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if (in.Op != midir.OpBoxDirect && in.Op != midir.OpUnboxDirect) || !f.Locals[in.Dst].Cross || f.Locals[in.Args[0]].Cross {
				continue
			}
			if a, ok := e.agg(in.Type); ok && a.counted() {
				out[in.Dst] = countedCross{src: in.Args[0], agg: a, unbox: in.Op == midir.OpUnboxDirect, hands: e.p.CrossHandsOver(f, &in), named: f.Locals[in.Args[0]].Ent != 0}
			}
		}
	}
	return out
}

func (e *emitter) settleCountedCross(sb *strings.Builder, f *midir.Func, in midir.Inst) {
	for i, a := range in.Args {
		if x, ok := e.countedX[a]; ok && x.unbox && e.keepsCrossTemp(f, in, i, x) {
			e.refCall(sb, e.direct[a], refDrop, aggPtr(a))
		}
	}
}

// An alias of a crossing out of a named local is a view the local keeps counted, and a move of it takes the local.
func (e *emitter) keepsCrossTemp(f *midir.Func, in midir.Inst, i int, x countedCross) bool {
	return in.Op == midir.OpShare || !e.p.ArgConsumed(f, in, i) || in.Op == midir.OpAlias && !x.hands
}

func (e *emitter) settleCountedSources(sb *strings.Builder, f *midir.Func, in midir.Inst) {
	for i, a := range in.Args {
		x, ok := e.countedX[a]
		switch {
		case !ok:
		case x.unbox:
			if in.Op == midir.OpMove && x.named && !e.keepsCrossTemp(f, in, i, x) {
				e.irb(sb).Call(rtRelease, vptr(e.load(sb, x.src)))
			}
		case e.p.BoxConsumed(in, i):
			e.dropCrossSource(sb, x, in.Op == midir.OpMove)
		}
	}
}

// A box consumed in place of its source took the source's counts with the copies it retained, so the source lets go of its own.
func (e *emitter) dropCrossSource(sb *strings.Builder, x countedCross, move bool) {
	if !x.unbox && (x.hands || move && x.named) {
		e.refCall(sb, x.agg, refDrop, aggPtr(x.src))
	}
}
