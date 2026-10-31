package mir

import (
	"fmt"
	"slices"

	"kigumi/internal/compiler/sem"
)

// ents renames the closures g builds to their copies.
func (p *Program) cloneFor(g *Func, ent sem.EntityID, name string, subst map[sem.EntityID]sem.TypeID, ents map[sem.EntityID]sem.EntityID) *Func {
	ty := func(t sem.TypeID) sem.TypeID { return p.R.Types.Subst(t, subst) }
	f := &Func{Name: name, Ent: ent, File: g.File, Ret: ty(g.Ret), ImplicitOk: g.ImplicitOk, rcBase: g.rcBase}
	f.Params = append([]LocalID(nil), g.Params...)
	f.PanicDrop = append([]LocalID(nil), g.PanicDrop...)
	f.Locals = make([]Local, len(g.Locals))
	for i, l := range g.Locals {
		l.Type = ty(l.Type)
		f.Locals[i] = l
	}
	f.Blocks = make([]Block, len(g.Blocks))
	for bid, blk := range g.Blocks {
		insts := make([]Inst, len(blk.Insts))
		for i, in := range blk.Insts {
			in.Args = append([]LocalID(nil), in.Args...)
			in.Type = ty(in.Type)
			if c, ok := ents[in.Ent]; ok && in.Op == OpClosure {
				in.Ent = c
			}
			// A Copy instance of a field `move` reads the field; the generic body could not know.
			if in.Op == OpFieldMove && p.R.IsCopy(f.Locals[in.Dst].Type) {
				in.Op = OpField
			}
			insts[i] = in
			if in.Op == OpBuiltin && in.Str == "comptime" {
				p.cloneComptime(g, f, BlockID(bid), in.Index, subst, ents)
			}
		}
		term := Term{Op: blk.Term.Op, Args: append([]LocalID(nil), blk.Term.Args...), Targets: append([]BlockID(nil), blk.Term.Targets...)}
		f.Blocks[bid] = Block{Insts: insts, Term: term}
	}
	return f
}

func (s *specializer) genericClosure(t sem.TypeID, seen map[sem.TypeID]bool) bool {
	if t == 0 || seen[t] {
		return false
	}
	seen[t] = true
	tt := s.p.R.Types
	n := tt.Node(t)
	if n.Kind == sem.KClosure {
		if len(tt.Params(s.p.R.Closure(n.Ent).Sig)) > 0 {
			return true
		}
		for _, c := range s.p.R.AggFields(t) {
			if len(tt.Params(c)) > 0 {
				return true
			}
		}
		return false
	}
	if s.genericClosure(n.Elem, seen) {
		return true
	}
	for _, a := range n.Args {
		if s.genericClosure(a, seen) {
			return true
		}
	}
	return false
}

// cloneComptime gives a specialised copy of a generic body its own comptime block, so the copy's placeholder gets folded too.
func (p *Program) cloneComptime(g, f *Func, bid BlockID, slot int, subst map[sem.EntityID]sem.TypeID, ents map[sem.EntityID]sem.EntityID) {
	for _, cf := range p.Comptime {
		if cf.Owner != g || cf.Block != bid || cf.Slot != slot {
			continue
		}
		name := fmt.Sprintf("%s.comptime%d", f.Name, slot)
		p.Comptime = append(p.Comptime, &ComptimeFunc{
			Site:   cf.Site,
			Func:   p.cloneFor(cf.Func, 0, name, subst, ents),
			Owner:  f,
			Block:  bid,
			Slot:   slot,
			Type:   p.R.Types.Subst(cf.Type, subst),
			Cloned: true,
		})
		return
	}
}

func (p *Program) dropStaleComptime() {
	live := make(map[*Func]bool, len(p.Funcs))
	for _, f := range p.Funcs {
		live[f] = true
	}
	p.Comptime = slices.DeleteFunc(p.Comptime, func(cf *ComptimeFunc) bool { return cf.Cloned && !live[cf.Owner] })
}
