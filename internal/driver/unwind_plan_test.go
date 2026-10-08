package driver_test

import (
	"slices"
	"testing"

	"kigumi/internal/mir"
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

func TestUnwindPlanIsEmptyForAsyncBodies(t *testing.T) {
	p := analyzeFixture(t, "async_task_panic_sync_deep.txtar")
	f := asyncFunc(t, p, "main.boom")
	plan := p.UnwindPlanOf(f)
	for bid, blk := range f.Blocks {
		for pc := range blk.Insts {
			if len(plan.SetAt(mir.BlockID(bid), pc)) != 0 {
				t.Fatalf("an async body carries an unwind set at block %d instruction %d", bid, pc)
			}
		}
	}
}
