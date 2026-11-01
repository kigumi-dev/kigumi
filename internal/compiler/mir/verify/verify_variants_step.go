package verify

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// fieldSlot keeps a field's path step apart from a payload slot's.
const fieldSlot = 1 << 24

func (vf *variantFlow) step(s *variantFacts, in midir.Inst) {
	p, f := vf.p, vf.f
	d := in.Dst
	var src midir.LocalID
	if len(in.Args) > 0 {
		src = in.Args[0]
	}
	srcBool := s.bools[src]
	srcPlace := s.where(src)
	s.forget(d)
	switch in.Op {
	case midir.OpIsVariant:
		kind, t := p.aggOf(f.Locals[src].Type)
		if kind != aggAdt || src == d || !vf.cond[d] {
			return
		}
		all := p.R.AggVariants(t)
		rest := variantSet{all: len(all)}
		for _, v := range all {
			if v != in.Ent {
				rest.list = append(rest.list, v)
			}
		}
		s.setBool(d, &boolFact{
			yes: &implied{facts: map[vplace]variantSet{srcPlace: {list: []sem.EntityID{in.Ent}, all: len(all)}}},
			no:  &implied{facts: map[vplace]variantSet{srcPlace: rest}},
		})
	case midir.OpPayload:
		if src != d && !vf.multi[d] {
			s.setAt(d, vplace{root: srcPlace.root, path: s.paths.child(srcPlace.path, in.Index)})
		}
	case midir.OpField, midir.OpFieldMove:
		if src != d && !vf.multi[d] {
			s.setAt(d, vplace{root: srcPlace.root, path: s.paths.child(srcPlace.path, fieldSlot|in.Index)})
		}
	case midir.OpAlias, midir.OpCopy, midir.OpMove, midir.OpShare, midir.OpBoxDirect, midir.OpUnboxDirect:
		if src == d {
			return
		}
		if vf.multi[d] {
			s.rebase(srcPlace, d)
		} else if !vf.inert[srcPlace.root] {
			s.setAt(d, srcPlace)
		}
		if srcBool != nil && vf.cond[d] {
			s.setBool(d, srcBool)
		}
	case midir.OpUnary:
		if in.Str == "!" && srcBool != nil && src != d && vf.cond[d] {
			s.setBool(d, &boolFact{yes: srcBool.no, no: srcBool.yes})
		}
	case midir.OpBinary:
		if in.Str == "<" && len(in.Args) == 2 && vf.cond[d] && in.Args[0] != d && in.Args[1] != d {
			lt := lessPlace(in.Args[0], in.Args[1])
			s.roots[in.Args[1]] = true
			s.setBool(d, &boolFact{yes: &implied{facts: map[vplace]variantSet{lt: assigned}}, no: &implied{}})
		}
	case midir.OpCellTake:
		s.setKnown(cellPlace(srcPlace), emptyCell)
	case midir.OpVariant:
		if kind, t := p.aggOf(f.Locals[d].Type); kind == aggAdt {
			s.setKnown(vplace{root: d}, variantSet{list: []sem.EntityID{in.Ent}, all: len(p.R.AggVariants(t))})
		}
	case midir.OpConst:
		if in.Type == sem.TyBool && in.Lit.Kind == sem.LitBool && vf.cond[d] {
			if in.Lit.Bool {
				s.setBool(d, &boolFact{yes: &implied{}, no: neverImplied})
			} else {
				s.setBool(d, &boolFact{yes: neverImplied, no: &implied{}})
			}
		}
	}
	if vf.cond[d] && s.bools[d] == nil && f.Locals[d].Type == sem.TyBool {
		s.setBool(d, &boolFact{yes: &implied{}, no: &implied{}})
	}
}
