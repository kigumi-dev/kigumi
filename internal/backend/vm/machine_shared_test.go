package vm

import (
	"sync"
	"sync/atomic"
	"testing"

	"kigumi/internal/compiler/mir"
)

// Several lanes hand the same value around, so the count has to survive
// concurrent retains and releases and free exactly once.
func TestConcurrentRetainAndReleaseFreeOnce(t *testing.T) {
	m := &Machine{sh: newShared()}
	const lanes, rounds = 8, 2000
	v := newObj(kInt)
	atomic.AddInt32(&v.rc, lanes)
	var wg sync.WaitGroup
	for range lanes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range rounds {
				retain(v)
				m.release(v)
			}
			m.release(v)
		}()
	}
	wg.Wait()
	if atomic.LoadInt32(&v.rc) != 1 {
		t.Fatalf("rc = %d after balanced traffic, want the one reference left", atomic.LoadInt32(&v.rc))
	}
	m.release(v)
	if v.k != kFree {
		t.Fatalf("the last release left k = %v, want the poisoned kFree", v.k)
	}
}

// A worker runs on its own Machine: the interpreter state starts empty and
// the shared tables stay the same ones.
func TestLaneStartsCleanAndKeepsSharedState(t *testing.T) {
	m := &Machine{sh: newShared(), host: map[string]hostFn{}}
	m.stack = []*mir.Func{{Name: "caller"}}
	m.depth, m.stepsLeft = 3, 9
	m.cur, m.curLane = &task{}, 2
	m.valueSites, m.boxFacts, m.boxParams = map[valueSiteKey]valueSite{}, map[*mir.Func]localFacts{}, mir.NewBoxedParams(nil)

	w := m.lane()
	if w.stack != nil || w.depth != 0 || w.stepsLeft != 0 || w.cur != nil || w.curLane != 0 || w.valueSites != nil || w.boxFacts != nil || w.boxParams != nil {
		t.Fatal("a lane inherited the interpreter state of the one that started it")
	}
	if w.sh != m.sh {
		t.Fatal("a lane must reach the same shared tables")
	}
	w.sh.globals[1] = unitObj
	if m.sh.globals[1] != unitObj {
		t.Fatal("a write through one lane must be visible through the other")
	}
}

// Every lane charges the same budget, so the live total must not lose updates.
func TestMemoryChargeIsSharedAcrossLanes(t *testing.T) {
	m := &Machine{sh: newShared(), opts: Options{Sandbox: true, MemoryLimit: 1 << 40}}
	const lanes, each = 8, 500
	var wg sync.WaitGroup
	for range lanes {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := m.lane()
			for range each {
				w.chargeMemory(mkBytes(make([]byte, 16)))
			}
		}()
	}
	wg.Wait()
	want := int64(lanes) * each * (16 + objOverhead)
	if got := m.sh.mem.Load(); got != want {
		t.Fatalf("live total = %d after %d charges, want %d", got, lanes*each, want)
	}
}
