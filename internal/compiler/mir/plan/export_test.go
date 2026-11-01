package plan

import (
	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
)

func CountlessRefs(p *mir.Program, f *midir.Func) []bool {
	return (&planner{Program: p}).countlessRefs(f)
}
