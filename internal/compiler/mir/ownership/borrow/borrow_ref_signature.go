package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (c *borrowCheck) signatureLifetimeSource(in midir.Inst) (*borrowInfo, bool) {
	sig := c.slotFnType(c.f.Locals[in.Args[0]].Type)
	if o, params, ok := c.valueResultCallee(in); ok {
		switch c.p.R.Entity(o.ent).Kind {
		case sem.EntFn:
			if b, ok := c.entityLifetimeSource(o.ent, c.p.R.Fn(o.ent), params, 0); ok {
				return b, true
			}
		case sem.EntClosure:
			sig = c.p.R.Closure(o.ent).Sig
		}
	}
	tt := c.p.R.Types
	shape := tt.Node(sig)
	lt := c.facts().DeclaredLifetime(shape.Elem)
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

// valueResultCallee is the one fn or closure value call in runs and the operands its body takes, as a direct call to
// it passes them.
func (c *borrowCheck) valueResultCallee(in midir.Inst) (calleeOrigin, []midir.LocalID, bool) {
	o, ok := c.soleOrigin(in.Args[0])
	if !ok {
		return calleeOrigin{}, nil, false
	}
	return o, o.params(in.Args[1:]), true
}
