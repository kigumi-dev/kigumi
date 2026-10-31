package llgen

import (
	"fmt"
	"slices"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
)

// siteAt is the instruction being emitted.
type siteAt struct {
	blk midir.BlockID
	pc  int
}

type outerDrops struct {
	sets  map[int32][]midir.LocalID
	order []int32
}

func (e *emitter) outerDropsFor(f *midir.Func) *outerDrops {
	if !e.catch || f.Async {
		return nil
	}
	return &outerDrops{sets: map[int32][]midir.LocalID{}}
}

func (e *emitter) dropCall(sb *strings.Builder, f *midir.Func, plain RuntimeFn, v string) {
	od := e.outer
	if od == nil {
		e.irb(sb).Call(plain, vptr(v))
		return
	}
	plan := e.p.UnwindPlanOf(f)
	live := e.sweepLocals(plan.SetAt(e.at.blk, e.at.pc))
	if len(live) == 0 {
		e.irb(sb).Call(rtDropOuter, vptr(v))
		return
	}
	idx := plan.At[e.at.blk][e.at.pc]
	if _, ok := od.sets[idx]; !ok {
		od.sets[idx] = live
		od.order = append(od.order, idx)
	}
	r := e.irb(sb).Call(rtReleaseOuter, vptr(v)).Text
	c, ok := e.newTmp(), e.newTmp()
	fmt.Fprintf(sb, "  %s = icmp ne i32 %s, 0\n", c, r)
	fmt.Fprintf(sb, "  br i1 %s, label %%ufail%d, label %%ucont%s\nucont%s:\n", c, idx, ok[2:], ok[2:])
}

func (e *emitter) sweepLocals(set []midir.LocalID) []midir.LocalID {
	var out []midir.LocalID
	for _, l := range set {
		if !e.isDirect(l) && !e.isScalar(l) {
			out = append(out, l)
		}
	}
	return out
}

func (e *emitter) outerFailBlocks(sb *strings.Builder) {
	od := e.outer
	if od == nil {
		return
	}
	slices.Sort(od.order)
	for _, idx := range od.order {
		fmt.Fprintf(sb, "ufail%d:\n", idx)
		for _, l := range od.sets[idx] {
			e.irb(sb).Call(rtSweepRelease, vptr(e.load(sb, l)))
		}
		e.irb(sb).Call(rtOuterFatal)
		sb.WriteString("  unreachable\n")
	}
}
