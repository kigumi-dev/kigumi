package driver_test

import (
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

func asyncFunc(t *testing.T, p *mir.Program, name string) *mir.Func {
	t.Helper()
	for _, f := range p.Funcs {
		if f.Name == name {
			return f
		}
	}
	t.Fatalf("no function %q in the program", name)
	return nil
}

// A frame holds what a resume reads and nothing else, and the cleanup list
// of a suspension point names what an abandoned frame releases.
func TestAsyncFramePlan(t *testing.T) {
	p := analyzeFixture(t, "async_cancel_await.txtar")
	w := asyncFunc(t, p, "main.worker")
	if !w.Async || w.Suspends != 1 {
		t.Fatalf("worker: async=%v suspends=%d, want an async body with one suspension point", w.Async, w.Suspends)
	}
	if w.Frame == 0 || len(w.Cleanup) != 2 {
		t.Fatalf("worker: frame=%d cleanup=%d, want slots and one cleanup list per suspension point", w.Frame, len(w.Cleanup))
	}
	named := map[string]bool{}
	for _, l := range w.Cleanup[1] {
		named[w.Locals[l].Name] = true
	}
	if !named["first"] || !named["second"] {
		t.Fatalf("worker cleanup at the await = %v, want both resources", named)
	}
	for i, l := range w.Locals {
		if w.Slot[i] >= 0 && w.Slot[i] >= w.Frame {
			t.Fatalf("local %s has slot %d outside a frame of %d", l.Name, w.Slot[i], w.Frame)
		}
	}
}

func TestAsyncMobility(t *testing.T) {
	p := analyzeFixture(t, "async_cancel_await.txtar")
	bound := p.R.Fn(asyncFunc(t, p, "main.worker").Ent)
	if bound.Mobility != sem.MobilityExecutorBound || bound.BoundBy == "" {
		t.Fatalf("worker: mobility=%v boundBy=%q, want executor-bound with the type that pins it", bound.Mobility, bound.BoundBy)
	}
	q := analyzeFixture(t, "async_await_shapes.txtar")
	free := q.R.Fn(asyncFunc(t, q, "main.loopSum").Ent)
	if free.Mobility != sem.MobilityMigratable || free.BoundBy != "" {
		t.Fatalf("loopSum: mobility=%v boundBy=%q, want migratable", free.Mobility, free.BoundBy)
	}
}
