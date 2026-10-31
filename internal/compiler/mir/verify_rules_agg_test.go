package mir_test

import (
	"testing"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

type ruleCase struct {
	name, fn, rule, fragment string
	edit                     func(t *testing.T, p *mir.Program, f *midir.Func)
}

func runRuleCases(t *testing.T, src string, plan bool, cases []ruleCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var p *mir.Program
			if plan {
				p = planned(t, src)
			} else {
				p = built(t, src)
			}
			c.edit(t, p, funcNamed(t, p, c.fn))
			wantRule(t, p, c.rule, c.fragment)
		})
	}
}

func newString(f *midir.Func) midir.LocalID {
	s := addLocal(f, "s", sem.TyString)
	f.Blocks[0].Insts = append([]midir.Inst{{Op: midir.OpConst, Dst: s, Type: sem.TyString, Lit: sem.Literal{Kind: sem.LitString}}}, f.Blocks[0].Insts...)
	return s
}

func TestVerifyAggregates(t *testing.T) {
	runRuleCases(t, rulesSrc, false, []ruleCase{
		{"field of a scalar", "main.add", "aggregates", "names a field of Int, which has none", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpField, Dst: addLocal(f, "x", sem.TyI64), Args: []midir.LocalID{0}})
		}},
		{"field of a sum type", "main.area", "aggregates", "a sum type", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpField, Dst: addLocal(f, "x", sem.TyF64), Args: []midir.LocalID{f.Params[0]}})
		}},
		{"field out of range", "main.shift", "aggregates", "names field 9 of Point, which has 2", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpField, 0).Index = 9
		}},
		{"field of another type", "main.shift", "aggregates", "reads a field of Int into String", func(t *testing.T, p *mir.Program, f *midir.Func) {
			f.Locals[nth(t, f, midir.OpField, 1).Dst].Type = sem.TyString
		}},
		{"stored field of another type", "main.lend", "aggregates", "stores String in a field of Int", func(t *testing.T, p *mir.Program, f *midir.Func) {
			in := nth(t, f, midir.OpSetField, 0)
			in.Args[1] = newString(f)
		}},
		{"payload of a scalar", "main.add", "aggregates", "which is not a sum type", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpPayload, Dst: addLocal(f, "x", sem.TyI64), Args: []midir.LocalID{0}})
		}},
		{"payload slot nobody has", "main.area", "aggregates", "payload slot 7, which no variant of Shape has", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpPayload, 0).Index = 7
		}},
		{"payload of another type", "main.area", "aggregates", "reads a payload of Shape into Int", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpPayload, Dst: addLocal(f, "x", sem.TyI64), Args: []midir.LocalID{f.Params[0]}})
		}},
		{"test of a variant the type lacks", "main.area", "aggregates", "that Shape does not have", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpIsVariant, 0).Ent = anyEntity(t, p, sem.EntVariant, "Some")
		}},
		{"tag that is no integer", "main.area", "aggregates", "yields String, not an integer", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpTag, Dst: addLocal(f, "x", sem.TyString), Args: []midir.LocalID{f.Params[0]}})
		}},
		{"variant of too few values", "main.mk", "aggregates", "builds Rect of 2 values from 1", func(t *testing.T, p *mir.Program, f *midir.Func) {
			in := nth(t, f, midir.OpVariant, 0)
			in.Args = in.Args[:1]
		}},
		{"variant payload of another type", "main.mk", "aggregates", "puts String in a payload of Float", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpVariant, 0).Args[0] = newString(f)
		}},
		{"record into a scalar", "main.add", "aggregates", "builds Point into Int", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpRecord, Dst: addLocal(f, "x", sem.TyI64), Ent: entityNamed(t, p, sem.EntType, "Point"), Args: []midir.LocalID{0, 1}})
		}},
		{"record field of another type", "main.shift", "aggregates", "puts String in a field of Int", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpRecord, 0).Args[0] = newString(f)
		}},
		{"array into a scalar", "main.add", "aggregates", "builds Int, which is not an array", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpArray, Dst: addLocal(f, "x", sem.TyI64)})
		}},
		{"array element of another type", "main.arr", "aggregates", "puts String in an array of Int", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpArray, Dst: addLocal(f, "x", f.Ret), Args: []midir.LocalID{newString(f)}})
		}},
		{"unbox of a scalar", "main.add", "aggregates", "which is not a boxed value", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpUnbox, Dst: addLocal(f, "x", sem.TyI64), Args: []midir.LocalID{0}})
		}},
		{"cell content of another type", "main.counter", "aggregates", "takes String out of a cell of Int", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpCellGet, Dst: addLocal(f, "x", sem.TyString), Args: []midir.LocalID{nth(t, f, midir.OpNewCell, 0).Dst}})
		}},
		{"stored cell content of another type", "main.counter", "aggregates", "stores String in a cell of Int", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpCellSet, Args: []midir.LocalID{nth(t, f, midir.OpNewCell, 0).Dst, newString(f)}})
		}},
		{"field moved out of a lent receiver", "main.Named.label", "aggregates", "moves a field out of %self", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpFieldMove, Dst: addLocal(f, "x", p.R.AggFields(paramType(f, 0))[0]), Args: []midir.LocalID{f.Params[0]}})
		}},
		{"copy field moved", "main.Point.total", "aggregates", "moves Int, a Copy type", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpFieldMove, Dst: addLocal(f, "x", sem.TyI64), Args: []midir.LocalID{f.Params[0]}})
		}},
	})
}

func TestVerifyCellOpsAndBuiltins(t *testing.T) {
	runRuleCases(t, rulesSrc, false, []ruleCase{
		{"cell read through a scalar", "main.add", "cellops", "which is no cell", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpCellGet, Dst: addLocal(f, "x", sem.TyI64), Args: []midir.LocalID{0}})
		}},
		{"unknown builtin", "main.add", "builtins", `no runtime entry "nope"`, func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpBuiltin, Str: "nope", Dst: addLocal(f, "x", sem.TyI64)})
		}},
		{"builtin of too few arguments", "main.add", "builtins", "print takes 1 to 1 arguments, got 0", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpBuiltin, Str: "print", Dst: addLocal(f, "x", sem.TyUnit)})
		}},
		{"iteration of a scalar", "main.add", "builtins", "iter.len walks Int", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpBuiltin, Str: "iter.len", Dst: addLocal(f, "x", sem.TyUsize), Args: []midir.LocalID{0}})
		}},
		{"iteration by a string", "main.sum", "builtins", "iter.at indexes with String", func(t *testing.T, p *mir.Program, f *midir.Func) {
			nth(t, f, midir.OpBuiltin, 1).Args[1] = newString(f)
		}},
		{"drop of a lent receiver", "main.Point.total", "droplent", "a reference this frame does not own", func(t *testing.T, p *mir.Program, f *midir.Func) {
			appendInst(f, midir.Inst{Op: midir.OpDrop, Args: []midir.LocalID{f.Params[0]}})
		}},
	})
}
