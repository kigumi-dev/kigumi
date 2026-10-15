package vm

import (
	"testing"

	"kigumi/internal/mir"
)

func TestSmallIntsAreSharedImmortals(t *testing.T) {
	a, b := mkInt(7, 64|256), mkInt(7, 64|256)
	if a != b || a.rc >= 0 {
		t.Fatalf("small ints should be one immortal object, got %p rc %d and %p rc %d", a, a.rc, b, b.rc)
	}
	if mkInt(7, 8) == a || mkInt(7, 64) == a {
		t.Fatal("ints of different kinds must not share an object")
	}
	if big := mkInt(1<<20, 64|256); big.rc != 1 {
		t.Fatalf("an int outside the table must be fresh, rc %d", big.rc)
	}
}

func TestRewriteIntLeavesSharedImmortalAlone(t *testing.T) {
	shared := mkInt(3, 64|256)
	r := mkRecord(0, []*obj{shared})
	rewriteInt(r, 0, 0)
	if shared.i != 3 || mkInt(3, 64|256).i != 3 {
		t.Fatalf("shared immortal was written through: %d", shared.i)
	}
	if got := r.fields[0].i; got != 0 {
		t.Fatalf("field = %d, want 0", got)
	}
	fresh := mkInt(1<<20, 64|256)
	r = mkRecord(0, []*obj{fresh})
	rewriteInt(r, 0, 5)
	if r.fields[0] != fresh || fresh.i != 5 {
		t.Fatal("a fresh int should be rewritten in place")
	}
}

func TestFreedScalarIsReused(t *testing.T) {
	m := &Machine{sh: newShared()}
	v := m.newInt(1<<20, 64|256)
	m.release(v)
	if v.k != kFree {
		t.Fatalf("freed scalar kind = %v, want kFree", v.k)
	}
	w := m.newInt(2<<20, 64|256)
	if w != v || w.k != kInt || w.i != 2<<20 || w.rc != 1 {
		t.Fatalf("freed scalar was not reused cleanly: %+v", w)
	}
}

func TestFreedScalarCountsBalance(t *testing.T) {
	TrackLive(true)
	defer TrackLive(false)
	m := &Machine{sh: newShared()}
	v := m.newInt(1<<20, 64|256)
	m.release(v)
	w := m.newInt(2<<20, 64|256)
	m.release(w)
	if n := LiveObjects(); n != 0 {
		t.Fatalf("%d objects live after balanced new and release", n)
	}
}

func TestFramesRecycleWithClearedLocals(t *testing.T) {
	m := &Machine{sh: newShared()}
	fr := m.newFrame(testFunc(4), nil)
	fr.locals[3] = mkInt(1<<20, 0)
	m.recycle(fr)
	again := m.newFrame(testFunc(6), nil)
	if again != fr {
		t.Fatal("frame was not reused")
	}
	for i, l := range again.locals {
		if l != nil {
			t.Fatalf("local %d not cleared", i)
		}
	}
}

func testFunc(locals int) *mir.Func { return &mir.Func{Locals: make([]mir.Local, locals)} }
