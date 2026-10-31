package mir

import (
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

type BoxedParams struct {
	p    *Program
	memo map[*midir.Func][]bool
}

func NewBoxedParams(p *Program) *BoxedParams {
	return &BoxedParams{p: p}
}

// A closure takes every scalar parameter boxed; a function does when its body stores it, calls a function value with it or passes it to another boxed parameter.
func (b *BoxedParams) Of(f *midir.Func) []bool {
	if b.memo == nil {
		b.solve()
	}
	if out, ok := b.memo[f]; ok {
		return out
	}
	return make([]bool, len(f.Params))
}

func (b *BoxedParams) solve() {
	b.memo = make(map[*midir.Func][]bool, len(b.p.Funcs))
	for _, f := range b.p.Funcs {
		b.memo[f] = make([]bool, len(f.Params))
	}
	for changed := true; changed; {
		changed = false
		for _, f := range b.p.Funcs {
			if f.Wraps != nil || f.Async {
				continue
			}
			out := b.memo[f]
			closure := f.Ent != 0 && b.p.R.Entity(f.Ent).Kind == sem.EntClosure
			for i, prm := range f.Params {
				if !out[i] && b.p.R.ScalarBoxes(f.Locals[prm].Type) && (closure || b.boxesUse(f, prm)) {
					out[i], changed = true, true
				}
			}
		}
	}
}

// A boxed parameter and its copies are passed on in the box they arrived in.
func (b *BoxedParams) Locals(f *midir.Func) []bool {
	out := make([]bool, len(f.Locals))
	for i, prm := range f.Params {
		if b.Of(f)[i] {
			for l := range family(f, prm) {
				out[l] = true
			}
		}
	}
	return out
}

func family(f *midir.Func, prm midir.LocalID) map[midir.LocalID]bool {
	fam := map[midir.LocalID]bool{prm: true}
	for grew := true; grew; {
		grew = false
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if copyLike(in.Op) && fam[in.Args[0]] && !fam[in.Dst] {
					fam[in.Dst], grew = true, true
				}
			}
		}
	}
	return fam
}

func (b *BoxedParams) boxesUse(f *midir.Func, prm midir.LocalID) bool {
	fam := family(f, prm)
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			for i, a := range in.Args {
				if fam[a] && b.boxedUse(f, in, i) {
					return true
				}
			}
		}
	}
	return false
}

func (b *BoxedParams) boxedUse(f *midir.Func, in midir.Inst, i int) bool {
	switch in.Op {
	case midir.OpCallValue:
		return i > 0 && in.Ent == 0
	case midir.OpCellSet:
		return i == 1
	case midir.OpSetField:
		return i == 1 && !f.IsDirect(in.Args[0])
	case midir.OpSetIndex:
		return i == 2 && !f.IsDirect(in.Args[0])
	case midir.OpCall:
		g := b.p.ByEnt[in.Ent]
		return g != nil && in.DPrim == opdef.DPNone && i < len(g.Params) && b.Of(g)[i]
	}
	return false
}

func copyLike(op midir.Opcode) bool {
	return op == midir.OpAlias || op == midir.OpShare || op == midir.OpMove
}

// Native computes these on registers and boxes the result only where it is stored or handed on.
func (p *Program) FloatConversion(f *midir.Func, in midir.Inst) bool {
	if in.Op != midir.OpCall || len(in.Args) != 1 || in.DPrim != opdef.DPNone || p.ByEnt[in.Ent] != nil || f.Locals[in.Dst].Type != sem.TyF64 {
		return false
	}
	e := p.R.Entity(in.Ent)
	if e.Kind != sem.EntFn || !strings.HasPrefix(e.Name, "to") || p.R.Fn(in.Ent).Owner == 0 || !strings.HasPrefix(StdKey(p.R, in.Ent, false), "std/prelude.") {
		return false
	}
	tt := p.R.Types
	self := f.Locals[in.Args[0]].Type
	if tt.Kind(self) == sem.KRef {
		self = tt.Node(self).Elem
	}
	return self == sem.TyF64 || tt.IsInteger(self) && self != sem.TyI128 && self != sem.TyU128
}
