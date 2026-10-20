package driver_test

import (
	"slices"
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

func callSets(p *mir.Program, f *mir.Func) map[string][]string {
	plan := p.UnwindPlanOf(f)
	out := map[string][]string{}
	for bid, blk := range f.Blocks {
		for pc, in := range blk.Insts {
			if in.Op != mir.OpCall {
				continue
			}
			var names []string
			for _, l := range plan.SetAt(mir.BlockID(bid), pc) {
				name := f.Locals[l].Name
				if f.Locals[l].Ent == 0 {
					name = ""
				}
				names = append(names, name)
			}
			out[p.R.Entity(in.Ent).Name] = names
		}
	}
	return out
}

func TestUnwindPlanNamesWhatEachCallLeavesOwned(t *testing.T) {
	p := analyzeFixture(t, "async_task_panic_sync_deep.txtar")
	b := callSets(p, asyncFunc(t, p, "main.b"))
	if got := b["c"]; !slices.Equal(got, []string{"r"}) {
		t.Errorf("b's call to c leaves %v owned, want the resource r", got)
	}
	c := callSets(p, asyncFunc(t, p, "main.c"))
	if got := c["explode"]; len(got) != 2 || !slices.Contains(got, "r") || !slices.Contains(got, "") {
		t.Errorf("c's call to explode leaves %v owned, want r and the Res already built", got)
	}
}

func TestUnwindPlanOfAsyncBodiesNamesFrameTemporaries(t *testing.T) {
	p := analyzeFixture(t, "async_task_panic_temps_await.txtar")
	f := asyncFunc(t, p, "main.inner")
	checkAsyncTemporaries(t, p, f, true)
	checkAsyncTemporaries(t, p, asyncFunc(t, p, "main.boom"), true)
	checkAsyncTemporaries(t, p, asyncFunc(t, p, "main.sib"), false)
}

func checkAsyncTemporaries(t *testing.T, p *mir.Program, f *mir.Func, want bool) {
	t.Helper()
	plan := p.UnwindPlanOf(f)
	temps := 0
	for _, set := range plan.Sets {
		for _, l := range set {
			if f.Locals[l].Ent != 0 {
				t.Errorf("the plan names the local %q, which the frame sweeps by name", f.Locals[l].Name)
			}
			if f.Slot[l] < 0 {
				t.Errorf("the plan names a temporary with no frame slot")
			}
			temps++
		}
	}
	if want && temps == 0 {
		t.Errorf("%s: the plan names no temporary live across a call that can panic", f.Name)
	}
	if !want && temps != 0 {
		t.Errorf("%s: the plan names %d temporaries, want none", f.Name, temps)
	}
}

func TestUnwindPlanSkipsLocalsThatOwnNothing(t *testing.T) {
	p := analyzeFixture(t, "async_task_panic_sync_deep.txtar")
	for _, f := range p.Funcs {
		for _, set := range p.UnwindPlanOf(f).Sets {
			for _, l := range set {
				if f.Locals[l].Type == sem.TyUnit {
					t.Errorf("%s: the plan names a unit-typed local, which owns no reference", f.Name)
				}
			}
		}
	}
}

func TestUnwindPlanLeavesALentReceiverToItsCaller(t *testing.T) {
	p := analyzeFixture(t, "async_task_panic_lent_receiver.txtar")
	got := callSets(p, asyncFunc(t, p, "main.Bag.scan"))["explode"]
	if !slices.Contains(got, "mine") || slices.Contains(got, "self") {
		t.Errorf("scan's call to explode leaves %v owned, want its own local mine and not the receiver it only borrows", got)
	}
	via := callSets(p, asyncFunc(t, p, "main.viaScan"))["scan"]
	if !slices.Contains(via, "bag") {
		t.Errorf("viaScan's call to scan leaves %v owned, want the bag it lent", via)
	}
}
