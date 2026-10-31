package driver

import (
	"fmt"
	"sort"

	resbound "kigumi/internal/compiler/mir/resource"
)

// ResourceReport lines, one per stack root sorted by name: the summary a
// hosted-growable target reports and a strict profile additionally gates on.
func ResourceReport(a *Analysis) []string {
	if a == nil || a.Prog == nil || a.Prog.Resources == nil {
		return nil
	}
	var lines []string
	var static uint64
	for _, ent := range resbound.StackRoots(a.Res) {
		f, s := a.Prog.ByEnt[ent], a.Prog.Resources[ent]
		if f == nil || s == nil {
			continue
		}
		lines = append(lines, a.Res.Entity(ent).Name+": "+describeStackBound(s.Chain))
		static = s.Static
	}
	lines = append(lines, asyncFrameLines(a)...)
	sort.Strings(lines)
	if static > 0 {
		lines = append(lines, fmt.Sprintf("static: %d bytes", static))
	}
	return lines
}

func describeStackBound(b resbound.StackBound) string {
	switch b.Kind {
	case resbound.Bounded:
		return fmt.Sprintf("bounded, %d bytes", b.Bytes)
	case resbound.Unbounded:
		return "unbounded (" + b.Reason + ")"
	default:
		return "unknown (" + b.Reason + ")"
	}
}

func asyncFrameLines(a *Analysis) []string {
	var lines []string
	for _, f := range a.Prog.Funcs {
		s := a.Prog.Resources[f.Ent]
		if !f.Async || s == nil || s.Async == nil || s.Async.Kind != resbound.Bounded {
			continue
		}
		if p := a.Res.Packages[a.Res.Entity(f.Ent).Pkg]; p.Std || p.Module != "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: async frame, %d bytes", a.Res.Entity(f.Ent).Name, s.Async.Bytes))
	}
	return lines
}
