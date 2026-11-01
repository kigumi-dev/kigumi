package plan

import (
	"fmt"
	"slices"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// ents renames the closures g builds to their copies.
func (p *planner) cloneFor(g *midir.Func, ent sem.EntityID, name string, subst map[sem.EntityID]sem.TypeID, ents map[sem.EntityID]sem.EntityID) *midir.Func {
	ty := func(t sem.TypeID) sem.TypeID { return p.R.Types.Subst(t, subst) }
	f := &midir.Func{Name: name, Ent: ent, File: g.File, Ret: ty(g.Ret), ImplicitOk: g.ImplicitOk, RCBase: g.RCBase}
	f.Params = append([]midir.LocalID(nil), g.Params...)
	f.PanicDrop = append([]midir.LocalID(nil), g.PanicDrop...)
	f.Locals = make([]midir.Local, len(g.Locals))
	for i, l := range g.Locals {
		l.Type = ty(l.Type)
		f.Locals[i] = l
	}
	f.Blocks = make([]midir.Block, len(g.Blocks))
	for bid, blk := range g.Blocks {
		insts := make([]midir.Inst, len(blk.Insts))
		for i, in := range blk.Insts {
			in.Args = append([]midir.LocalID(nil), in.Args...)
			in.Type = ty(in.Type)
			if c, ok := ents[in.Ent]; ok && in.Op == midir.OpClosure {
				in.Ent = c
			}
			// A Copy instance of a field `move` reads the field; the generic body could not know.
			if in.Op == midir.OpFieldMove && p.R.IsCopy(f.Locals[in.Dst].Type) {
				in.Op = midir.OpField
			}
			insts[i] = in
			if in.Op == midir.OpBuiltin && in.Str == "comptime" {
				p.cloneComptime(g, f, midir.BlockID(bid), in.Index, subst, ents)
			}
		}
		term := midir.Term{Op: blk.Term.Op, Args: append([]midir.LocalID(nil), blk.Term.Args...), Targets: append([]midir.BlockID(nil), blk.Term.Targets...)}
		f.Blocks[bid] = midir.Block{Insts: insts, Term: term}
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
func (p *planner) cloneComptime(g, f *midir.Func, bid midir.BlockID, slot int, subst map[sem.EntityID]sem.TypeID, ents map[sem.EntityID]sem.EntityID) {
	for _, cf := range p.Comptime {
		if cf.Owner != g || cf.Block != bid || cf.Slot != slot {
			continue
		}
		name := fmt.Sprintf("%s.comptime%d", f.Name, slot)
		p.Comptime = append(p.Comptime, &mir.ComptimeFunc{
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

func (p *planner) dropStaleComptime() {
	live := make(map[*midir.Func]bool, len(p.Funcs))
	for _, f := range p.Funcs {
		live[f] = true
	}
	p.Comptime = slices.DeleteFunc(p.Comptime, func(cf *mir.ComptimeFunc) bool { return cf.Cloned && !live[cf.Owner] })
}
