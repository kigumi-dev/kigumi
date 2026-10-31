package mir

import "kigumi/internal/compiler/sem"

func (vf *variantFlow) block(bid BlockID, entry *variantFacts, check bool) (*variantFacts, *Error) {
	s := entry
	if !check {
		s = entry.clone()
	}
	f := vf.f
	blk := &f.Blocks[bid]
	for i, in := range blk.Insts {
		if check {
			if a, bad := s.unassigned(vf.weak, in); bad {
				return nil, fail("variants", bid, i, "%s reads %%%s, which is not assigned on every path that reaches it", in.Op, f.Locals[a].Name)
			}
			if in.Op == OpPayload {
				if msg := vf.payloadVariants(s, in); msg != "" {
					return nil, fail("variants", bid, i, "%s", msg)
				}
			}
			if a, bad := s.emptied(in); bad {
				return nil, fail("variants", bid, i, "%s uses the cell %%%s after celltake emptied it", in.Op, f.Locals[a].Name)
			}
			if in.Op == OpBuiltin && iterAt(in.Str) && len(in.Args) == 2 && !vf.inBounds(s, in) {
				return nil, fail("variants", bid, i, "%s indexes %%%s by %%%s on a path that never tested it against the length", in.Str, f.Locals[in.Args[0]].Name, f.Locals[in.Args[1]].Name)
			}
		}
		if in.Op == OpCellSet {
			s.drop([]vplace{cellPlace(s.where(in.Args[0]))})
		}
		if in.Op == OpSetField {
			s.forgetBelow(s.where(in.Args[0]).root)
		}
		if in.Op.Yields() {
			vf.step(s, in)
			if vf.weak != nil && vf.weak[in.Dst] {
				s.setKnown(assignedPlace(in.Dst), assigned)
			}
		}
		if gone, ok := leavesSlot(in); ok && vf.weak != nil && vf.weak[gone] {
			s.forget(gone)
		}
	}
	if a, bad := s.unassigned(vf.weak, Inst{Op: OpNop, Args: blk.Term.Args}); bad && check {
		return nil, fail("variants", bid, len(blk.Insts), "terminator reads %%%s, which is not assigned on every path that reaches it", f.Locals[a].Name)
	}
	return s, nil
}

func leavesSlot(in Inst) (LocalID, bool) {
	switch {
	case in.Op == OpDrop:
		return in.Args[0], true
	case in.Op == OpMove && in.Args[0] != in.Dst:
		return in.Args[0], true
	}
	return 0, false
}

var assigned = variantSet{list: []sem.EntityID{1}, all: 2}

func assignedPlace(l LocalID) vplace { return vplace{root: l, tag: tagAssigned} }

// unassigned finds a read of a local not known to be assigned here; dropping a
// never-assigned local drops a nulled slot, which is nothing.
func (s *variantFacts) unassigned(weak []bool, in Inst) (LocalID, bool) {
	if weak == nil || in.Op == OpDrop || in.Op == OpRelease {
		return 0, false
	}
	for _, a := range in.Args {
		if weak[a] {
			if set, ok := s.known[assignedPlace(a)]; !ok || len(set.list) != 1 {
				return a, true
			}
		}
	}
	return 0, false
}

func (vf *variantFlow) payloadVariants(s *variantFacts, in Inst) string {
	p, f := vf.p, vf.f
	x := in.Args[0]
	kind, t := p.aggOf(f.Locals[x].Type)
	if kind != aggAdt {
		return ""
	}
	possible, ok := s.known[s.where(x)]
	list := possible.list
	if !ok {
		list = p.R.AggVariants(t)
	}
	for _, v := range list {
		if n := len(p.R.Variant(v).Payload); in.Index >= n {
			return "reads payload slot " + itoa(in.Index) + " of %" + f.Locals[x].Name + ", which may hold " + p.R.Entity(v).Name + " with " + itoa(n)
		}
	}
	return ""
}
