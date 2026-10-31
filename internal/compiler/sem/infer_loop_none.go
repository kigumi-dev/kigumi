package sem

import "kigumi/internal/compiler/syntax"

func (c *checker) isNoneName(n syntax.NodeID) bool {
	if c.t.Kind(n) != syntax.Ident {
		return false
	}
	h := c.identHead(n)
	return h.kind == headVariant && c.r.Entities[h.ent].Name == "None" && c.r.Entities[h.ent].Parent == c.r.Types.optionEnt
}

func (c *checker) mergeBareNone(b *loopBreaks) TypeID {
	best := TyNever
	if len(b.types) > 0 {
		best = c.merge(b.nodes, b.types)
	}
	if _, isOpt := c.r.Types.IsOption(best); !isOpt {
		best = 0
	}
	for _, n := range b.bareNone {
		var t TypeID
		if best != 0 {
			t = c.check(n, best)
		} else {
			t = c.synth(n)
		}
		b.nodes, b.types = append(b.nodes, n), append(b.types, t)
	}
	if best != 0 {
		return best
	}
	return c.merge(b.nodes, b.types)
}
