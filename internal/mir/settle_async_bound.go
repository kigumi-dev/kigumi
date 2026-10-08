package mir

import "kigumi/internal/sem"

// asyncBound is Unknown until SettleAsyncBounds reads the frame PlanDirect lays out.
func asyncBound(r *sem.Result, f *Func) *StackBound {
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

func SettleAsyncBounds(prog *Program) {
	for _, f := range prog.Funcs {
		if s := prog.Resources[f.Ent]; s != nil && s.Async != nil && f.Async && f.Slot != nil {
			s.Async = &StackBound{Kind: Bounded, Bytes: AsyncFrameBytes(prog.R, f)}
		}
	}
}
