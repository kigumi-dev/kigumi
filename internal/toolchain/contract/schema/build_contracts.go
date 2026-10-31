package schema

import "kigumi/internal/compiler/sem"

// contractsOf summarizes a function or method entity's effects and
// operations from sem's own inference, never re-derived from its source.
func contractsOf(res *sem.Result, ent sem.EntityID) *Contracts {
	fn := res.Fn(ent)
	summary := res.Summary(ent)
	ops := res.OpSummaryOf(ent)
	c := &Contracts{
		DeclaredPure:    fn.Declared&sem.EffPure != 0,
		DeclaredNoalloc: fn.Declared&sem.EffNoalloc != 0,
		DeclaredAsync:   fn.Declared&sem.EffAsync != 0,
		DeclaredUnsafe:  fn.Declared&sem.EffUnsafe != 0,
		InferredPure:    summary.Pure,
		InferredNoalloc: summary.Noalloc,
		Unresolved:      ops.Unresolved,
	}
	for _, f := range ops.Facts {
		c.Operations = append(c.Operations, f.Fact)
	}
	return c
}
