package mir

import (
	"slices"

	"kigumi/internal/sem"
)

// paramCopies maps a declared `noalloc` generic body to the type parameters it copies; such a copy allocates only for some arguments.
type paramCopies map[sem.EntityID][]int

func (c paramCopies) add(ent sem.EntityID, k int) bool {
	if slices.Contains(c[ent], k) {
		return false
	}
	c[ent] = append(c[ent], k)
	return true
}

func (p *Program) paramCopy(f *Func, in Inst) (int, bool) {
	if in.Op != OpShare || !p.declaredNoalloc(f) {
		return 0, false
	}
	return p.ownParamIndex(f, f.Locals[in.Args[0]].Type)
}

func (p *Program) ownParamIndex(f *Func, t sem.TypeID) (int, bool) {
	if f.Ent == 0 || p.R.Types.Kind(t) != sem.KParam {
		return 0, false
	}
	k := slices.Index(p.R.Fn(f.Ent).TypeParams, p.R.Types.Node(t).Ent)
	return k, k >= 0
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
					if in.Op != OpCall || len(facts[in.Ent]) == 0 {
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

func (p *Program) chargeCall(f *Func, in Inst, facts paramCopies) (sites []sem.CopySite, grown bool) {
	args, bound := p.bindTypeArgs(f, in, func(t sem.TypeID) sem.TypeID { return t })
	tps := p.R.Fn(in.Ent).TypeParams
	for _, k := range facts[in.Ent] {
		t := p.R.Types.Param(tps[k])
		if bound {
			t = args[k]
		}
		if bound && !p.R.CopyAllocates(t) {
			continue
		}
		if j, own := p.ownParamIndex(f, t); own && p.declaredNoalloc(f) {
			grown = facts.add(f.Ent, j) || grown
			continue
		}
		sites = append(sites, sem.CopySite{Owner: f.Ent, File: f.File, Node: p.siteNode(f, in), Type: t, Inst: true})
	}
	return sites, grown
}
