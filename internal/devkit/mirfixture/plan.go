package mirfixture

import (
	"testing"

	"kigumi/internal/compiler/mir"
	mirbuild "kigumi/internal/compiler/mir/build"
	midir "kigumi/internal/compiler/mir/ir"
	mirplan "kigumi/internal/compiler/mir/plan"
)

func Planned(t *testing.T, src string) *mir.Program {
	t.Helper()
	prog := mirbuild.Program(CheckSourceMir(t, src))
	if prog.HasErrors() {
		t.Fatalf("build errors:\n%s", prog.Render())
	}
	passes := []mirplan.Transform{
		{Name: "plan-rc", Run: mirplan.PlanRC},
		{Name: "plan-direct", Run: mirplan.PlanDirect},
	}
	if err := mirplan.Apply(prog, passes...); err != nil {
		t.Fatalf("passes: %v\n%s", err, prog.Dump())
	}
	return prog
}

func FuncNamed(t *testing.T, p *mir.Program, name string) *midir.Func {
	t.Helper()
	for _, f := range p.Funcs {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("no function %s", name)
	return nil
}

func CountOp(f *midir.Func, op midir.Opcode) int {
	n := 0
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == op {
				n++
			}
		}
	}
	return n
}

func FirstOp(f *midir.Func, op midir.Opcode) (midir.Inst, bool) {
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op == op {
				return in, true
			}
		}
	}
	return midir.Inst{}, false
}

func LocalNamed(t *testing.T, f *midir.Func, name string) midir.LocalID {
	t.Helper()
	for i, l := range f.Locals {
		if l.Name == name && l.Ent != 0 {
			return midir.LocalID(i)
		}
	}
	t.Fatalf("%s has no local %s", f.Name, name)
	return 0
}
