package mir

import "kigumi/internal/compiler/sem"

func (c *borrowCheck) signatureLifetimeSource(in Inst) (*borrowInfo, bool) {
	sig := c.slotFnType(c.f.Locals[in.Args[0]].Type)
	if e := c.fnEntityOf(in.Args[0]); e != 0 {
		switch c.p.R.Entity(e).Kind {
		case sem.EntFn:
			if b, ok := c.entityLifetimeSource(e, c.p.R.Fn(e), in.Args, 1); ok {
				return b, true
			}
		case sem.EntClosure:
			sig = c.p.R.Closure(e).Sig
		}
	}
	tt := c.p.R.Types
	shape := tt.Node(sig)
	lt := c.declaredLifetime(shape.Elem)
	if lt == 0 {
		return nil, false
	}
	var candidates []lifetimeCandidate
	for i, t := range shape.Args {
		if n := tt.Node(tt.RefLifetime(t)); n.Kind == sem.KParam && n.Ent == lt {
			candidates = append(candidates, lifetimeCandidate{arg: i + 1})
		}
	}
	return c.resolveLifetimeHolder(0, in.Args, candidates)
}
