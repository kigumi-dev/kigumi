package llgen

import (
	"fmt"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
)

func (e *emitter) label(kind string) string {
	e.tmp++
	return fmt.Sprintf("%s%d", kind, e.tmp)
}

func (e *emitter) tryFixedInst(sb *strings.Builder, f *midir.Func, in midir.Inst) bool {
	if in.Op == midir.OpBuiltin && in.Str == "fixed.fill" {
		a, ok := e.direct[in.Dst]
		if ok {
			e.fixedFill(sb, f, a, in)
		}
		return ok
	}
	if len(in.Args) == 0 {
		return false
	}
	a, ok := e.direct[in.Args[0]]
	if !ok || a.Arr == nil {
		return false
	}
	switch {
	case in.Op == midir.OpIndex:
		e.loadLeaf(sb, f, *a.Arr, e.elemPtr(sb, f, in, a, in.Args[0], in.Args[1], !in.NoTrap), in.Dst)
	case in.Op == midir.OpSetIndex:
		e.storeLeaf(sb, *a.Arr, e.elemPtr(sb, f, in, a, in.Args[0], in.Args[1], !in.NoTrap), in.Args[2])
	case in.Op == midir.OpBuiltin && in.Str == "iter.len":
		e.storeScalarResult(sb, f, in.Dst, Value{TI64, fmt.Sprint(a.Len)})
	case in.Op == midir.OpBuiltin && strings.HasPrefix(in.Str, "iter.at"):
		e.loadLeaf(sb, f, *a.Arr, e.elemPtr(sb, f, in, a, in.Args[0], in.Args[1], false), in.Dst)
	default:
		return false
	}
	return true
}

func (e *emitter) elemPtr(sb *strings.Builder, f *midir.Func, in midir.Inst, a *aggInfo, base, idx midir.LocalID, checked bool) string {
	iv, _ := e.scalarOperand(sb, f, idx)
	if checked {
		bad, ok, c := e.label("oob"), e.label("inb"), e.newTmp()
		fmt.Fprintf(sb, "  %s = icmp uge i64 %s, %d\n  br i1 %s, label %%%s, label %%%s\n%s:\n", c, iv.Text, a.Len, c, bad, ok, bad)
		e.irb(sb).Call(rtPanicIndex, vi64text(iv.Text), vi64(int(a.Len)), vptr(e.siteText(f, in)))
		fmt.Fprintf(sb, "  unreachable\n%s:\n", ok)
	}
	return e.elemAt(sb, a, aggPtr(base), iv.Text)
}

func (e *emitter) fixedFill(sb *strings.Builder, f *midir.Func, a *aggInfo, in midir.Inst) {
	fld := *a.Arr
	if fld.Unit || a.Len == 0 {
		return
	}
	if fld.Agg != nil {
		e.countLoop(sb, a.Len, func(j string) {
			copyAgg(sb, fld.Agg, e.elemAt(sb, a, aggPtr(in.Dst), j), aggPtr(in.Args[0]))
		})
		return
	}
	v, _ := e.scalarOperand(sb, f, in.Args[0])
	e.countLoop(sb, a.Len, func(j string) {
		e.storeLeafScalar(sb, fld, e.elemAt(sb, a, aggPtr(in.Dst), j), v.Text)
	})
}
