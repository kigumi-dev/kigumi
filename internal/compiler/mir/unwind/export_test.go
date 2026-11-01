package unwind

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func ImmortalLocals(f *midir.Func, r *sem.Result) map[midir.LocalID]bool { return immortalLocals(f, r) }
