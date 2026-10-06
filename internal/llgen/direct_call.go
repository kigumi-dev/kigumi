package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
)

func (e *emitter) aggArg(sb *strings.Builder, a *aggInfo, l mir.LocalID) string {
	v := ""
	if _, ok := e.direct[l]; ok {
		v = e.loadAgg(sb, a, l)
	} else {
		p := e.load(sb, l)
		v = e.unboxAgg(sb, a, p)
		e.irb(sb).Call(rtRelease, vptr(p))
	}
	if byValueAgg(a) {
		return a.Name + " " + v
	}
	slot := e.stackSlot(sb, a.Name)
	fmt.Fprintf(sb, "  store %s %s, ptr %s\n", a.Name, v, slot)
	return "ptr " + slot
}

func (e *emitter) directCall(sb *strings.Builder, f *mir.Func, in mir.Inst, fn *mir.Func, s *aggSig) {
	var args []string
	out := ""
	if s.Ret != nil && !byValueAgg(s.Ret) {
		out = e.stackSlot(sb, s.Ret.Name)
		args = append(args, "ptr "+out)
	}
	for i, l := range in.Args {
		if a := s.param(i); a != nil {
			args = append(args, e.aggArg(sb, a, l))
			continue
		}
		if e.isScalar(l) {
			args = append(args, "ptr "+e.scalarBoxOnDemand(sb, l))
			continue
		}
		if ag, ok := e.direct[l]; ok {
			args = append(args, "ptr "+e.boxAgg(sb, ag, e.loadAgg(sb, ag, l)))
			continue
		}
		args = append(args, "ptr "+e.load(sb, l))
	}
	if s.Ret == nil {
		t := e.newTmp()
		fmt.Fprintf(sb, "  %s = call ptr %s(%s)\n", t, e.sym(fn), strings.Join(args, ", "))
		if !e.storeDirectResult(sb, in.Dst, t) {
			e.storeMaybeScalar(sb, in.Dst, t)
		}
		return
	}
	v := out
	if byValueAgg(s.Ret) {
		v = e.newTmp()
		fmt.Fprintf(sb, "  %s = call %s %s(%s)\n", v, s.Ret.Name, e.sym(fn), strings.Join(args, ", "))
	} else {
		fmt.Fprintf(sb, "  call void %s(%s)\n", e.sym(fn), strings.Join(args, ", "))
		v = e.newTmp()
		fmt.Fprintf(sb, "  %s = load %s, ptr %s\n", v, s.Ret.Name, out)
	}
	if d, ok := e.direct[in.Dst]; ok {
		e.storeAgg(sb, d, in.Dst, v)
		return
	}
	e.store(sb, in.Dst, e.boxAgg(sb, s.Ret, v))
}

func (e *emitter) returnAgg(sb *strings.Builder, a *aggInfo, l mir.LocalID) {
	v := ""
	if _, ok := e.direct[l]; ok {
		v = e.loadAgg(sb, a, l)
	} else {
		p := e.load(sb, l)
		v = e.unboxAgg(sb, a, p)
		e.irb(sb).Call(rtRelease, vptr(p))
	}
	if byValueAgg(a) {
		fmt.Fprintf(sb, "  ret %s %s\n", a.Name, v)
		return
	}
	fmt.Fprintf(sb, "  store %s %s, ptr %%sret\n  ret void\n", a.Name, v)
}
