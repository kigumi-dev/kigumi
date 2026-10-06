package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
)

func (e *emitter) term(sb *strings.Builder, t mir.Term) {
	switch t.Op {
	case mir.TermJump:
		fmt.Fprintf(sb, "  br label %%b%d\n", t.Targets[0])
	case mir.TermBranch:
		if info, ok := e.scalar[t.Args[0]]; ok && info.Ty == TI1 {
			c := e.loadNative(sb, t.Args[0])
			fmt.Fprintf(sb, "  br i1 %s, label %%b%d, label %%b%d\n", c.Text, t.Targets[0], t.Targets[1])
			return
		}
		v := e.load(sb, t.Args[0])
		c := e.irb(sb).Call(rtTruth, vptr(v)).Text
		fmt.Fprintf(sb, "  br i1 %s, label %%b%d, label %%b%d\n", c, t.Targets[0], t.Targets[1])
	case mir.TermReturn:
		if e.sig != nil && e.sig.Ret != nil {
			e.returnAgg(sb, e.sig.Ret, t.Args[0])
			return
		}
		if a, ok := e.direct[t.Args[0]]; ok {
			fmt.Fprintf(sb, "  ret ptr %s\n", e.boxAgg(sb, a, e.loadAgg(sb, a, t.Args[0])))
			return
		}
		if info, ok := e.scalar[t.Args[0]]; ok {
			boxed, lit := e.litSlot(t.Args[0])
			if !lit {
				boxed = e.boxNative(sb, info, e.loadNative(sb, t.Args[0]))
			}
			fmt.Fprintf(sb, "  ret ptr %s\n", boxed)
			return
		}
		v := e.load(sb, t.Args[0])
		fmt.Fprintf(sb, "  ret ptr %s\n", v)
	default:
		sb.WriteString("  unreachable\n")
	}
}
