package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// paramCopies maps a declared `noalloc` generic body to the type parameters it copies; such a copy allocates only for some arguments.
type paramCopies map[sem.EntityID][]paramCopyFact

// paramCopyFact is a copy of type parameter k; elem marks an array element handed out in an Option.
type paramCopyFact struct {
	k    int
	elem bool
}

func (c paramCopies) add(ent sem.EntityID, fact paramCopyFact) bool {
	if slices.Contains(c[ent], fact) {
		return false
	}
	c[ent] = append(c[ent], fact)
	return true
}

func (p *Program) paramCopy(f *midir.Func, in midir.Inst) (paramCopyFact, bool) {
	if !p.declaredNoalloc(f) {
		return paramCopyFact{}, false
	}
	if elem, ok := p.elemCopy(f, in); ok {
		k, own := p.ownParamIndex(f, elem)
		return paramCopyFact{k: k, elem: true}, own
	}
	if in.Op != midir.OpShare {
		return paramCopyFact{}, false
	}
	k, own := p.ownParamIndex(f, f.Locals[in.Args[0]].Type)
	return paramCopyFact{k: k}, own
}

func (p *Program) ownParamIndex(f *midir.Func, t sem.TypeID) (int, bool) {
	if f.Ent == 0 || p.R.Types.Kind(t) != sem.KParam {
		return 0, false
	}
	k := slices.Index(p.R.Fn(f.Ent).TypeParams, p.R.Types.Node(t).Ent)
	return k, k >= 0
}

// boundCopy is what the shared body allocates for t, or 0; its element op always boxes the Option.
func (p *Program) boundCopy(fact paramCopyFact, t sem.TypeID) sem.TypeID {
	switch {
	case p.R.CopyAllocates(t):
		return t
	case fact.elem:
		return p.R.Types.Option(t)
	}
	return 0
}

func (p *Program) chargeCalls(facts paramCopies) []sem.CopySite {
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
func (p *Program) erasesToNoalloc(f *midir.Func, in midir.Inst) bool {
	return (in.Op == midir.OpFnItem || in.Op == midir.OpBind) && p.R.ErasesToNoalloc(f.File, in.Node)
}

// A specialized f reuses its generic's call info, so its own substitution binds in's type arguments.
func (p *Program) chargeArgs(f *midir.Func, in midir.Inst) ([]sem.TypeID, bool) {
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

func (p *Program) chargeCall(f *midir.Func, in midir.Inst, facts paramCopies) (sites []sem.CopySite, grown bool) {
	args, bound := p.chargeArgs(f, in)
	tps := p.R.Fn(in.Ent).TypeParams
	for _, fact := range facts[in.Ent] {
		t := p.R.Types.Param(tps[fact.k])
		if bound {
			t = args[fact.k]
		}
		if j, own := p.ownParamIndex(f, t); own && p.declaredNoalloc(f) {
			grown = facts.add(f.Ent, paramCopyFact{k: j, elem: fact.elem}) || grown
			continue
		}
		if bound {
			if t = p.boundCopy(fact, t); t == 0 {
				continue
			}
		}
		site := sem.CopySite{Owner: f.Ent, File: f.File, Node: p.siteNode(f, in), Type: t, Inst: true}
		if in.Op != midir.OpCall {
			site.Inst, site.Erased = false, in.Ent
		}
		sites = append(sites, site)
	}
	return sites, grown
}
