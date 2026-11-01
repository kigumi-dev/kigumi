package verify_test

import (
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/mirfixture"
)

const directRulesSrc = mirfixture.DirectPointSrc + `
type Pair = {
    pub a Int
    pub b Int
}

fn area(p: Point) -> Int { p.x * p.y }

fn mkPoint(x: Int) -> Point { Point { x: x, y: x } }

fn mkPair(x: Int) -> Pair { Pair { a: x, b: x } }
`

func TestVerifyDirectSignature(t *testing.T) {
	runRuleCases(t, directRulesSrc, true, []ruleCase{
		{"signature of too few parameters", "main.area", "direct", "direct signature of 0 parameters for a function of 1", func(t *testing.T, p *mir.Program, f *midir.Func) {
			f.DSig.Params = nil
		}},
		{"parameter of another type", "main.area", "direct", "direct signature takes String as parameter 0", func(t *testing.T, p *mir.Program, f *midir.Func) {
			f.DSig.Params[0] = sem.TyString
		}},
		{"result of another type", "main.mkPoint", "direct", "direct signature returns Pair for a function of Point", func(t *testing.T, p *mir.Program, f *midir.Func) {
			f.DSig.Ret = entityType(t, p, "Pair")
		}},
	})
}

func TestVerifyCrossLocals(t *testing.T) {
	runRuleCases(t, directRulesSrc, true, []ruleCase{
		{"crossing flag on a parameter", "main.area", "direct", "the crossing local %p", func(t *testing.T, p *mir.Program, f *midir.Func) {
			f.Locals[f.Params[0]].Cross = true
		}},
	})
}

func TestVerifyFixedBox(t *testing.T) {
	runRuleCases(t, directRulesSrc, true, []ruleCase{
		{"fixed box of a computed value", "main.intoArray", "direct", "as fixed, but it is not a constant", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpBoxDirect, 0).Fixed = true
		}},
	})
}

func entityType(t *testing.T, p *mir.Program, name string) sem.TypeID {
	t.Helper()
	for _, f := range p.Funcs {
		for _, l := range f.Locals {
			if p.R.TypeString(l.Type) == name {
				return l.Type
			}
		}
	}
	t.Fatalf("no local of type %s", name)
	return 0
}

func TestVerifyDirectForms(t *testing.T) {
	runRuleCases(t, directRulesSrc, true, []ruleCase{
		{"parameter in the wrong form", "main.mkPoint", "direct", "in the form this position does not take", func(t *testing.T, p *mir.Program, f *midir.Func) {
			f.Direct[f.Params[0]] = true
		}},
		{"value written in the wrong form", "main.area", "direct", "in the form this position does not yield", func(t *testing.T, p *mir.Program, f *midir.Func) {
			dst := nth(t, f, midir.OpField, 0).Dst
			f.Direct[dst] = !f.Direct[dst]
		}},
		{"box that does not change the form", "main.intoArray", "direct", "must read the one form and yield the other", func(t *testing.T, p *mir.Program, f *midir.Func) {
			in := nth(t, f, midir.OpBoxDirect, 0)
			f.Direct[in.Dst] = true
		}},
		{"box of another type", "main.intoArray", "types", "of Pair holds Point", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpBoxDirect, 0).Type = entityType(t, p, "Pair")
		}},
		{"box carried as a scalar", "main.intoArray", "types", "carried as Int", func(t *testing.T, p *mir.Program, f *midir.Func) {
			f.Locals[nth(t, f, midir.OpBoxDirect, 0).Dst].Type = sem.TyI64
		}},
		{"crossing local set twice", "main.intoArray", "direct", "named or set more than once", func(t *testing.T, p *mir.Program, f *midir.Func) {
			f.Locals[nth(t, f, midir.OpBoxDirect, 0).Dst].Ent = anyEntity(t, p, sem.EntFn, "mkPoint")
		}},
	})
}

func TestVerifyEnvSignature(t *testing.T) {
	runRuleCases(t, mirfixture.ClosureEnvSrc, true, []ruleCase{
		{"closure body without its signature", "main.local.closure1", "direct", "has no direct signature", func(t *testing.T, p *mir.Program, f *midir.Func) {
			f.DSig = nil
		}},
		{"capture left boxed", "main.local.closure1", "direct", "leaves capture", func(t *testing.T, p *mir.Program, f *midir.Func) {
			for i := range f.DSig.Params {
				f.DSig.Params[i] = 0
			}
		}},
	})
}

func TestVerifyAsyncDirectSignature(t *testing.T) {
	runRuleCases(t, asyncRulesSrc, true, []ruleCase{
		{"signature on an async body", "main.twice", "direct", "passes everything boxed", func(t *testing.T, p *mir.Program, f *midir.Func) {
			f.DSig = &midir.DirectSig{Params: make([]sem.TypeID, len(f.Params))}
		}},
	})
}
