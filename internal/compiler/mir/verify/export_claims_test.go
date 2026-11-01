package verify

import (
	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
)

func IsrInertArgs(p *mir.Program, f *midir.Func, in midir.Inst, g *midir.Func) []bool {
	return (&claims{Program: p}).isrInertArgs(f, in, g, nil)
}
