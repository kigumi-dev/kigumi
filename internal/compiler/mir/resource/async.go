package resource

import (
	midir "kigumi/internal/compiler/mir/ir"
	mirlayout "kigumi/internal/compiler/mir/layout"
	"kigumi/internal/compiler/sem"
)

// asyncBound is Unknown until SettleAsync reads the frame PlanDirect lays out.
func asyncBound(r *sem.Result, f *midir.Func) *StackBound {
	e := r.Entity(f.Ent)
	async := false
	switch e.Kind {
	case sem.EntFn, sem.EntImplicitMain, sem.EntTest, sem.EntContract:
		async = r.Fn(f.Ent).Declared&sem.EffAsync != 0
	}
	if !async {
		return nil
	}
	b := StackBound{Kind: Unknown, Reason: "the async frame is laid out once the MIR plans have run"}
	return &b
}

func SettleAsync(r *sem.Result, funcs []*midir.Func, summaries map[sem.EntityID]*Summary) {
	for _, f := range funcs {
		if s := summaries[f.Ent]; s != nil && s.Async != nil && f.Async && f.Slot != nil {
			s.Async = &StackBound{Kind: Bounded, Bytes: mirlayout.AsyncFrameBytes(r, f)}
		}
	}
}
