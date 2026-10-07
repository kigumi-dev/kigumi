package sem

// A closure's captures can still grow while context requirements settle, so the answer is never memoized.
func (r *Result) directClosure(t TypeID) bool {
	ent := r.Types.Node(t).Ent
	if ent == 0 || r.Entities[ent].Kind != EntClosure {
		return false
	}
	caps := r.closure(ent).Captures
	if len(caps) == 0 {
		return false
	}
	for _, c := range caps {
		if c.Mode != capCopy || !r.directLeaf(r.Entities[c.Local].Type) {
			return false
		}
	}
	return true
}

func (r *Result) closureEnv(ent EntityID) []TypeID {
	caps := r.closure(ent).Captures
	out := make([]TypeID, len(caps))
	for i, c := range caps {
		out[i] = r.Entities[c.Local].Type
	}
	return out
}

func (r *Result) containsClosure(t TypeID) bool {
	n := r.Types.Node(t)
	switch n.Kind {
	case KClosure:
		return true
	case KNamed:
		for _, a := range n.Args {
			if r.containsClosure(a) {
				return true
			}
		}
	}
	return false
}

func (r *Result) DirectClosure(t TypeID) bool {
	return r.Types.Kind(t) == KClosure && r.directClosure(t)
}
