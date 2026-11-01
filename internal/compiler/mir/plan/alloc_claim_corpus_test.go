package plan_test

import (
	"testing"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
)

// A body the summary calls noalloc, declared or inferred, holds no instruction that allocates once planned.
func checkNoallocBodies(t *testing.T, res *sem.Result, prog *mir.Program) {
	t.Helper()
	if res.HasErrors() {
		t.Fatalf("planning broke a declaration:\n%s", res.Render())
	}
	for _, f := range prog.Funcs {
		if f.Ent == 0 || !res.Summary(f.Ent).Noalloc {
			continue
		}
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if prog.InstAllocates(f, in) {
					t.Errorf("%s claims noalloc but holds an allocating %s\n%s", f.Name, in.Op, prog.Dump())
					return
				}
			}
		}
	}
}
