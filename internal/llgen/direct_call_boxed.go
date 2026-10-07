package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
)

func (e *emitter) callBoxed(sb *strings.Builder, f *mir.Func, vals []string) string {
	s := e.directSig(f)
	if s == nil {
		t := e.newTmp()
		fmt.Fprintf(sb, "  %s = call ptr %s(%s)\n", t, e.sym(f), joinArgs(vals))
		return t
	}
	var args []string
	out := ""
	if s.Ret != nil && !byValueAgg(s.Ret) {
		out = e.stackSlot(sb, s.Ret.Name)
		args = append(args, "ptr "+out)
	}
	for i, v := range vals {
		a := s.param(i)
		if a == nil {
			args = append(args, "ptr "+v)
			continue
		}
		slot := e.stackSlot(sb, a.Name)
		e.unboxAgg(sb, a, slot, v)
		// A lent position never owned the box; nothing to release.
		if !e.p.ParamLent(f.Ent, i) {
			e.irb(sb).Call(rtRelease, vptr(v))
		}
		if byValueAgg(a) {
			args = append(args, a.Name+" "+e.aggValue(sb, a, slot))
			continue
		}
		args = append(args, "ptr "+slot)
	}
	if s.Ret == nil {
		t := e.newTmp()
		fmt.Fprintf(sb, "  %s = call ptr %s(%s)\n", t, e.sym(f), strings.Join(args, ", "))
		return t
	}
	if byValueAgg(s.Ret) {
		v := e.newTmp()
		fmt.Fprintf(sb, "  %s = call %s %s(%s)\n", v, s.Ret.Name, e.sym(f), strings.Join(args, ", "))
		out = e.stackSlot(sb, s.Ret.Name)
		fmt.Fprintf(sb, "  store %s %s, ptr %s\n", s.Ret.Name, v, out)
	} else {
		fmt.Fprintf(sb, "  call void %s(%s)\n", e.sym(f), strings.Join(args, ", "))
	}
	return e.boxAgg(sb, s.Ret, out)
}
