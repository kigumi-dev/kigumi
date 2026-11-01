package plan

import (
	"slices"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// paramCopies maps a declared `noalloc` generic body to the type parameters it copies; such a copy allocates only for some arguments.
type paramCopies map[sem.EntityID][]mir.ParamCopyFact

func (c paramCopies) add(ent sem.EntityID, fact mir.ParamCopyFact) bool {
	if slices.Contains(c[ent], fact) {
		return false
	}
	c[ent] = append(c[ent], fact)
	return true
}

// boundCopy is what the shared body allocates for t, or 0; its element op always boxes the Option.
func (p *planner) boundCopy(fact mir.ParamCopyFact, t sem.TypeID) sem.TypeID {
	switch {
	case p.R.CopyAllocates(t):
		return t
	case fact.Elem:
		return p.R.Types.Option(t)
	}
	return 0
}

func (p *planner) chargeCalls(facts paramCopies) []sem.CopySite {
	var sites []sem.CopySite
	for changed := len(facts) > 0; changed; {
		changed, sites = false, sites[:0]
		for _, f := range p.Funcs {
			if f.Ent == 0 {
				continue
			}
			for _, blk := range f.Blocks {
				for _, in := range blk.Insts {
					if len(facts[in.Ent]) == 0 || in.Op != midir.OpCall && !p.erasesToNoalloc(f, in) {
						continue
					}
					more, grown := p.chargeCall(f, in, facts)
					sites, changed = append(sites, more...), changed || grown
				}
			}
		}
	}
	return sites
}

// erasesToNoalloc reads sem's coercion target, since a fn item value keeps the item's own effects.
func (p *planner) erasesToNoalloc(f *midir.Func, in midir.Inst) bool {
	return (in.Op == midir.OpFnItem || in.Op == midir.OpBind) && p.R.ErasesToNoalloc(f.File, in.Node)
}

// A specialized f reuses its generic's call info, so its own substitution binds in's type arguments.
func (p *planner) chargeArgs(f *midir.Func, in midir.Inst) ([]sem.TypeID, bool) {
	subst := func(t sem.TypeID) sem.TypeID { return t }
	if inst, ok := p.R.InstanceOf(f.Ent); ok {
		sub := p.R.InstanceSubst(inst.Gen, inst.Args)
		subst = func(t sem.TypeID) sem.TypeID { return p.R.Types.Subst(t, sub) }
	}
	if in.Op != midir.OpBind {
		return p.bindTypeArgs(f, in, subst)
	}
	info := p.R.Fn(in.Ent)
	want, bound := map[sem.EntityID]bool{}, map[sem.EntityID]sem.TypeID{}
	for _, tp := range info.TypeParams {
		want[tp] = !p.R.IsLifetimeParam(tp)
	}
	if !p.R.Types.BindParams(info.Sig, f.Locals[in.Dst].Type, want, bound) {
		return nil, false
	}
	return p.boundArgs(in.Ent, want, bound)
}

func (p *planner) chargeCall(f *midir.Func, in midir.Inst, facts paramCopies) (sites []sem.CopySite, grown bool) {
	args, bound := p.chargeArgs(f, in)
	tps := p.R.Fn(in.Ent).TypeParams
	for _, fact := range facts[in.Ent] {
		t := p.R.Types.Param(tps[fact.K])
		if bound {
			t = args[fact.K]
		}
		if j, own := p.OwnParamIndex(f, t); own && p.DeclaredNoalloc(f) {
			grown = facts.add(f.Ent, mir.ParamCopyFact{K: j, Elem: fact.Elem}) || grown
			continue
		}
		if bound {
			if t = p.boundCopy(fact, t); t == 0 {
				continue
			}
		}
		site := sem.CopySite{Owner: f.Ent, File: f.File, Node: p.SiteNode(f, in), Type: t, Inst: true}
		if in.Op != midir.OpCall {
			site.Inst, site.Erased = false, in.Ent
		}
		sites = append(sites, site)
	}
	return sites, grown
}
