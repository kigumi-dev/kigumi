package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
)

func (e *emitter) loadAgg(sb *strings.Builder, a *aggInfo, l mir.LocalID) string {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = load %s, ptr %%l%d\n", t, a.Name, l)
	return t
}

func (e *emitter) storeAgg(sb *strings.Builder, a *aggInfo, l mir.LocalID, v string) {
	fmt.Fprintf(sb, "  store %s %s, ptr %%l%d\n", a.Name, v, l)
}

func (e *emitter) tryDirectInst(sb *strings.Builder, f *mir.Func, in mir.Inst) bool {
	switch in.Op {
	case mir.OpRecord:
		if a, ok := e.direct[in.Dst]; ok {
			e.buildAgg(sb, f, a, in)
			return true
		}
	case mir.OpField, mir.OpFieldMove:
		if a, ok := e.direct[in.Args[0]]; ok {
			e.readAggField(sb, f, a, in)
			return true
		}
	case mir.OpSetField:
		if a, ok := e.direct[in.Args[0]]; ok {
			fld := a.Fields[in.Index]
			v := e.aggLeaf(sb, f, fld, in.Args[1], true)
			g := e.newTmp()
			fmt.Fprintf(sb, "  %s = getelementptr %s, ptr %%l%d, i32 0, i32 %d\n", g, a.Name, in.Args[0], in.Index)
			fmt.Fprintf(sb, "  store %s %s, ptr %s\n", fld.llType(), v, g)
			return true
		}
	case mir.OpAlias, mir.OpMove, mir.OpCopy, mir.OpShare:
		if a, ok := e.direct[in.Dst]; ok {
			e.storeAgg(sb, a, in.Dst, e.loadAgg(sb, a, in.Args[0]))
			return true
		}
	case mir.OpCall:
		if fn, ok := e.p.ByEnt[in.Ent]; ok {
			if s := e.directSig(fn); s != nil {
				e.directCall(sb, f, in, fn, s)
				return true
			}
		}
	case mir.OpDrop, mir.OpRelease:
		// A pure direct value owns no reference and has no drop glue.
		return e.isDirect(in.Args[0])
	case mir.OpBoxDirect:
		if a, ok := e.agg(in.Type); ok {
			e.store(sb, in.Dst, e.boxAgg(sb, a, e.loadAgg(sb, a, in.Args[0])))
			return true
		}
	case mir.OpUnboxDirect:
		if a, ok := e.agg(in.Type); ok {
			p := e.load(sb, in.Args[0])
			v := e.unboxAgg(sb, a, p)
			e.irb(sb).Call(rtRelease, vptr(p))
			e.storeAgg(sb, a, in.Dst, v)
			return true
		}
	}
	return false
}

func (e *emitter) buildAgg(sb *strings.Builder, f *mir.Func, a *aggInfo, in mir.Inst) {
	cur := "undef"
	for i, fld := range a.Fields {
		v := e.aggLeaf(sb, f, fld, in.Args[i], true)
		t := e.newTmp()
		fmt.Fprintf(sb, "  %s = insertvalue %s %s, %s %s, %d\n", t, a.Name, cur, fld.llType(), v, i)
		cur = t
	}
	e.storeAgg(sb, a, in.Dst, cur)
}

func (e *emitter) readAggField(sb *strings.Builder, f *mir.Func, a *aggInfo, in mir.Inst) {
	fld := a.Fields[in.Index]
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = extractvalue %s %s, %d\n", t, a.Name, e.loadAgg(sb, a, in.Args[0]), in.Index)
	if fld.Agg == nil {
		e.storeScalarResult(sb, f, in.Dst, Value{fld.Ty, t})
		return
	}
	if d, ok := e.direct[in.Dst]; ok {
		e.storeAgg(sb, d, in.Dst, t)
		return
	}
	e.store(sb, in.Dst, e.boxAgg(sb, fld.Agg, t))
}

// consume says the position takes over the reference, so a box read
// through here is released.
func (e *emitter) aggLeaf(sb *strings.Builder, f *mir.Func, fld aggField, l mir.LocalID, consume bool) string {
	if fld.Agg != nil {
		if a, ok := e.direct[l]; ok {
			return e.loadAgg(sb, a, l)
		}
		p := e.load(sb, l)
		v := e.unboxAgg(sb, fld.Agg, p)
		if consume {
			e.irb(sb).Call(rtRelease, vptr(p))
		}
		return v
	}
	if _, ok := e.scalar[l]; ok {
		return e.loadNative(sb, l).Text
	}
	p := e.load(sb, l)
	v := e.unboxScalar(sb, fld.Ty, p)
	if consume {
		e.irb(sb).Call(rtRelease, vptr(p))
	}
	return v.Text
}

func (e *emitter) instArgs(sb *strings.Builder, in mir.Inst) ([]string, []bool) {
	args := make([]string, 0, len(in.Args))
	boxed := make([]bool, len(in.Args))
	for i, a := range in.Args {
		if e.isScalar(a) {
			// classifyArgs only lets a scalar-classified local reach here
			// at a non-retaining read; released below, in releaseScalarArgs.
			args = append(args, e.scalarBoxOnDemand(sb, a))
			continue
		}
		args = append(args, e.load(sb, a))
		boxed[i] = e.boxTmp[a]
	}
	return args, boxed
}

func (e *emitter) storeDirectResult(sb *strings.Builder, dst mir.LocalID, res string) bool {
	a, ok := e.direct[dst]
	if !ok {
		return false
	}
	v := e.unboxAgg(sb, a, res)
	e.irb(sb).Call(rtRelease, vptr(res))
	e.storeAgg(sb, a, dst, v)
	return true
}
