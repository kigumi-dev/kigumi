package instance

import (
	"sync"

	"kigumi/internal/compiler/sem"
)

// Canon turns types into runtime identities interned in the type table: every lifetime is the one stand-in, and a
// closure type is its source closure applied to the instantiation it was built for. The zero value is ready.
type Canon struct {
	mu   sync.Mutex
	memo map[sem.TypeID]sem.TypeID
}

func (c *Canon) Of(r *sem.Result, t sem.TypeID) sem.TypeID {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.memo == nil {
		c.memo = map[sem.TypeID]sem.TypeID{}
	}
	return c.of(r, t)
}

func (c *Canon) of(r *sem.Result, t sem.TypeID) sem.TypeID {
	if t == 0 {
		return 0
	}
	if u, ok := c.memo[t]; ok {
		return u
	}
	tt := r.Types
	n := tt.Node(t)
	u := t
	switch n.Kind {
	case sem.KParam:
		if r.IsLifetimeParam(n.Ent) {
			u = tt.Param(r.LifetimeStandIn(n.Ent))
		}
	case sem.KNamed, sem.KIface, sem.KFn:
		u = tt.Rebuild(t, c.all(r, n.Args), c.of(r, n.Elem))
	case sem.KRef, sem.KPtr:
		u = tt.RebuildRef(t, c.of(r, n.Elem))
	case sem.KClosure:
		u = c.closure(r, t, n.Ent, n.Args)
	}
	c.memo[t] = u
	c.memo[u] = u
	return u
}

func (c *Canon) all(r *sem.Result, ts []sem.TypeID) []sem.TypeID {
	out := make([]sem.TypeID, len(ts))
	for i, t := range ts {
		out[i] = c.of(r, t)
	}
	return out
}

// closure is the identity of closure type ent: its source closure applied to closureScope as the copy ent was
// minted under, or to those parameters themselves for the source.
func (c *Canon) closure(r *sem.Result, t sem.TypeID, ent sem.EntityID, args []sem.TypeID) sem.TypeID {
	if len(args) > 0 {
		return r.Types.ClosureInstance(ent, c.all(r, args))
	}
	origin := r.ClosureOrigin(ent)
	scope := closureScope(r, origin)
	if len(scope) == 0 {
		return r.Types.Closure(origin)
	}
	sub := r.ClosureSubst(ent)
	out := make([]sem.TypeID, len(scope))
	for i, p := range scope {
		out[i] = c.of(r, r.Types.Subst(r.Types.Param(p), sub))
	}
	return r.Types.ClosureInstance(origin, out)
}

// Params are the type and const parameters t's runtime identity depends on, lifetimes left out: those it names,
// and for a source closure those closureScope lists.
func Params(r *sem.Result, t sem.TypeID) []sem.EntityID {
	var out []sem.EntityID
	seen := map[sem.EntityID]bool{}
	var walk func(sem.TypeID)
	walk = func(t sem.TypeID) {
		if t == 0 {
			return
		}
		n := r.Types.Node(t)
		switch {
		case n.Kind == sem.KParam:
			if !seen[n.Ent] && !r.IsLifetimeParam(n.Ent) {
				seen[n.Ent] = true
				out = append(out, n.Ent)
			}
			return
		case n.Kind == sem.KClosure && len(n.Args) == 0:
			origin := r.ClosureOrigin(n.Ent)
			sub := r.ClosureSubst(n.Ent)
			for _, p := range closureScope(r, origin) {
				walk(r.Types.Subst(r.Types.Param(p), sub))
			}
			return
		}
		walk(n.Elem)
		for _, a := range n.Args {
			walk(a)
		}
	}
	walk(t)
	return out
}

// Concrete reports whether t names one runtime type: no inference variable and no parameter but a lifetime.
func Concrete(r *sem.Result, t sem.TypeID) bool {
	return t != 0 && !r.Types.ContainsVar(t) && len(Params(r, t)) == 0
}
