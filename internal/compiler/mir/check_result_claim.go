package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// resultClaims names the boxes a body causes in another frame: a boxed
// callee's entry or the callee behind a value or a method table.
func (p *Program) resultClaims(f *midir.Func, code string) []AllocClaim {
	c, ok := sem.CodeByName(code)
	if !ok {
		return nil
	}
	var out []AllocClaim
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if t := p.boxedResult(f, in); t != 0 {
				out = append(out, p.claimAt(f, c, in.Node, t))
			}
		}
	}
	return out
}

func (p *Program) boxedResult(f *midir.Func, in midir.Inst) sem.TypeID {
	switch in.Op {
	case midir.OpCallValue:
		if f.IsDirect(in.Args[0]) {
			return 0
		}
	case midir.OpCall:
		if p.R.Entity(p.R.Entity(in.Ent).Parent).Kind != sem.EntInterface {
			return 0
		}
	case midir.OpFnItem, midir.OpBind, midir.OpClosure:
		if g := p.ByEnt[in.Ent]; g != nil && !in.Async && (in.Op != midir.OpClosure || in.Str == "method") {
			return p.entryBox(g)
		}
		return 0
	default:
		return 0
	}
	if t := f.Locals[in.Dst].Type; p.R.BoxAllocates(t) {
		return t
	}
	return 0
}

func (p *Program) ResultBoxed(f *midir.Func, in midir.Inst) bool {
	switch in.Op {
	case midir.OpCallValue:
		return !f.IsDirect(in.Args[0])
	case midir.OpCall:
		return p.R.Entity(p.R.Entity(in.Ent).Parent).Kind == sem.EntInterface
	}
	return false
}

// entryBox is the type g's entry allocates a box for, 0 when it allocates none.
func (p *Program) entryBox(g *midir.Func) sem.TypeID {
	if g.Entry == nil {
		return 0
	}
	for _, blk := range g.Entry.Blocks {
		for _, in := range blk.Insts {
			if in.Op == midir.OpBoxDirect && !in.Fixed && p.R.BoxAllocates(in.Type) {
				return in.Type
			}
		}
	}
	return 0
}

func (p *Program) namedCallees() map[sem.EntityID]bool {
	out := map[sem.EntityID]bool{}
	for _, f := range p.allFuncs() {
		if f.Wraps != nil {
			continue
		}
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				switch in.Op {
				case midir.OpCall, midir.OpFnItem, midir.OpBind, midir.OpClosure:
					out[in.Ent] = true
				}
			}
		}
	}
	return out
}

// A free generic function every caller reaches through an instance never runs its shared body.
func (p *Program) sharedUnreached(f *midir.Func, reached map[sem.EntityID]bool) bool {
	if f.Ent == 0 || reached[f.Ent] || p.R.Entity(f.Ent).Kind != sem.EntFn {
		return false
	}
	info := p.R.Fn(f.Ent)
	return len(info.TypeParams) > 0 && info.Recv == sem.RecvNone && info.Owner == 0 && f != p.Entry
}
