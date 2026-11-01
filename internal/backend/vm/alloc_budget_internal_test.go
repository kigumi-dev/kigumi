package vm

import (
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func chargeUntilAbort(t *testing.T, m *Machine) (n int) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("chargeStep never aborted")
		}
		p, ok := r.(*Panic)
		if !ok || !p.Sandbox || !strings.HasPrefix(p.Msg, "comptime block exceeded its step limit") {
			t.Fatalf("recovered %v, want the sandbox step-budget Panic", r)
		}
	}()
	for {
		m.chargeStep()
		n++
	}
}

func TestStepBudgetIsSharedAcrossLanes(t *testing.T) {
	const limit, lanes = 10_000, 8
	m := &Machine{sh: newShared(), opts: Options{Sandbox: true, StepLimit: limit}}
	var wg sync.WaitGroup
	var total atomic.Int64
	for range lanes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			total.Add(int64(chargeUntilAbort(t, m.lane())))
		}()
	}
	wg.Wait()
	if got := m.sh.steps.Load(); got != limit {
		t.Fatalf("shared step total = %d, want exactly the %d-step limit", got, limit)
	}
	if got := total.Load(); got > limit {
		t.Fatalf("lanes ran %d steps, want at most the %d-step limit", got, limit)
	}
}

func TestStepBudgetAbortsOnFirstStepPastLimit(t *testing.T) {
	for _, limit := range []int{1, stepBatch - 1, stepBatch, stepBatch + 1, 1000, 5_000} {
		m := &Machine{sh: newShared(), opts: Options{Sandbox: true, StepLimit: limit}}
		if n := chargeUntilAbort(t, m); n != limit {
			t.Fatalf("StepLimit %d: %d steps passed before the abort, want %d", limit, n, limit)
		}
	}
}
