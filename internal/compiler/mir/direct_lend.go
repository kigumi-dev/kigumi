package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	mirlayout "kigumi/internal/compiler/mir/layout"
	"kigumi/internal/compiler/sem"
)

// Past this a by-value copy per call costs more than the box it saves.
const maxLendBytes = 128

type lendParam struct {
	f *midir.Func
	i int
}

// A shared reference parameter its body only reads takes the value, and a caller lends a local by passing it as is.
func (p *Program) lendByValue() {
	cands := p.lendCandidates()
	if len(cands) == 0 {
		return
	}
	for c := range cands {
		elem := p.R.Types.Node(c.f.Locals[c.f.Params[c.i]].Type).Elem
		for l := range p.refAliases(c.f, c.f.Params[c.i]) {
			c.f.Locals[l].Type = elem
			c.f.Locals[l].Borrowed = c.f.Locals[l].Ent != 0
		}
	}
	for _, f := range p.allFuncs() {
		p.passBorrowedLocals(f, cands)
	}
}

func (p *Program) lendCandidates() map[lendParam]bool {
	cands := map[lendParam]bool{}
	for _, f := range p.allFuncs() {
		for i := range f.Params {
			if p.lendableParam(f, i) {
				cands[lendParam{f, i}] = true
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for c := range cands {
			if !p.readsOnly(c, cands) {
				delete(cands, c)
				changed = true
			}
		}
	}
	return cands
}

func (p *Program) lendableParam(f *midir.Func, i int) bool {
	if f.Async || f.Ent == 0 {
		return false
	}
	prm := f.Params[i]
	l := f.Locals[prm]
	tt := p.R.Types
	if l.Cell || l.Borrowed || tt.Kind(l.Type) != sem.KRef || tt.RefMut(l.Type) || !p.R.DirectAgg(tt.Node(l.Type).Elem) {
		return false
	}
	if size, _ := mirlayout.AggLayout(p.R, tt.Node(l.Type).Elem); size > maxLendBytes {
		return false
	}
	return p.R.LendableParamSlot(f.Ent, l.Ent) && !p.pinnedByPanic(f, prm)
}

func (p *Program) pinnedByPanic(f *midir.Func, l midir.LocalID) bool {
	for _, d := range f.PanicDrop {
		if d == l {
			return true
		}
	}
	return false
}

// refAliases names the parameter and the locals that only rename it.
func (p *Program) refAliases(f *midir.Func, prm midir.LocalID) map[midir.LocalID]bool {
	set := map[midir.LocalID]bool{prm: true}
	for changed := true; changed; {
		changed = false
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if in.Op == midir.OpAlias && set[in.Args[0]] && !set[in.Dst] {
					set[in.Dst], changed = true, true
				}
			}
		}
	}
	return set
}

// readsOnly holds when every use reads a field or passes the parameter to one that takes the value too.
func (p *Program) readsOnly(c lendParam, cands map[lendParam]bool) bool {
	f, prm := c.f, c.f.Params[c.i]
	set := p.refAliases(f, prm)
	defs := map[midir.LocalID]int{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op.Yields() && set[in.Dst] {
				defs[in.Dst]++
			}
		}
	}
	for l := range set {
		if (l == prm && defs[l] != 0) || (l != prm && defs[l] != 1) {
			return false
		}
	}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			for k, a := range in.Args {
				if set[a] && !(in.Op == midir.OpAlias && set[in.Dst]) && !p.readsAt(f, in, k, cands) {
					return false
				}
			}
		}
		for _, a := range blk.Term.Args {
			if set[a] {
				return false
			}
		}
	}
	return true
}

// A counted value lent by its bits holds none of its counts, so nothing may drop it or take it by value.
func (p *Program) readsAt(f *midir.Func, in midir.Inst, k int, cands map[lendParam]bool) bool {
	t := f.Locals[in.Args[k]].Type
	if p.R.Types.Kind(t) == sem.KRef {
		t = p.R.Types.Node(t).Elem
	}
	counted := p.R.CountedAdt(t)
	switch in.Op {
	case midir.OpDrop, midir.OpRelease:
		return k == 0 && !counted
	case midir.OpField, midir.OpPayload, midir.OpIsVariant, midir.OpTag, midir.OpNop:
		return k == 0
	case midir.OpBuiltin:
		return protoBuiltin(in)
	case midir.OpCall:
		g := p.ByEnt[in.Ent]
		if g == nil || k >= len(g.Params) || p.IsStdPrimitive(in.Ent) {
			return false
		}
		return cands[lendParam{g, k}] || !counted && p.takesValue(g, k, f.Locals[in.Args[k]].Type)
	}
	return false
}

// takesValue reports whether g's parameter k is the direct aggregate the reference points at.
func (p *Program) takesValue(g *midir.Func, k int, ref sem.TypeID) bool {
	prm := g.Params[k]
	t := g.Locals[prm].Type
	return p.R.Types.Kind(ref) == sem.KRef && t == p.R.Types.Node(ref).Elem && p.R.DirectAgg(t) && p.R.DirectParamSlot(g.Ent, g.Locals[prm].Ent) && !g.Async && !p.EnvABI(g)
}

func (p *Program) passBorrowedLocals(f *midir.Func, cands map[lendParam]bool) {
	defs, uses := map[midir.LocalID]*midir.Inst{}, map[midir.LocalID]int{}
	for bid := range f.Blocks {
		blk := &f.Blocks[bid]
		for i := range blk.Insts {
			in := &blk.Insts[i]
			if in.Op == midir.OpBorrow && in.Index == 0 {
				defs[in.Dst] = in
			}
			for _, a := range in.Args {
				uses[a]++
			}
		}
		for _, a := range blk.Term.Args {
			uses[a]++
		}
	}
	for bid := range f.Blocks {
		for i := range f.Blocks[bid].Insts {
			in := &f.Blocks[bid].Insts[i]
			if in.Op != midir.OpCall {
				continue
			}
			g := p.ByEnt[in.Ent]
			for k, a := range in.Args {
				d := defs[a]
				if d == nil || g == nil || uses[a] != 1 || !cands[lendParam{g, k}] {
					continue
				}
				if src := d.Args[0]; !f.Locals[src].Cell && p.R.DirectAgg(f.Locals[src].Type) {
					in.Args = replaceArg(in.Args, k, src)
					*d = midir.Inst{Op: midir.OpNop, Node: d.Node}
				}
			}
		}
	}
}
