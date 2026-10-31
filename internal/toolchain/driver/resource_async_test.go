package driver_test

import (
	"fmt"
	"slices"
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/toolchain/driver"
)

func TestResourceReportAsyncFrame(t *testing.T) {
	t.Parallel()
	src := "import task from std/task\n\ntype Box = {\n    pub a Int\n    pub b Int\n    pub c Int\n    pub d Int\n}\n\nasync fn twice(n: Int) -> Int {\n    n * 2\n}\n\nasync fn walk(n: Int) -> Int {\n    let box = Box { a: n, b: 1, c: 2, d: 3 }\n    let v = await twice(box.a)\n    box.d + v\n}\n\nlet ex = task.currentThreadExecutor()\nprint \"${ex.runToCompletion(walk(3))}\"\n"
	a := analyzeModule(t, src, false, 0)
	if a.Res.Types.PtrBits() != 64 {
		t.Skip("the expected frame is the 64-bit kgframe")
	}
	report := driver.ResourceReport(a)
	var walk *midir.Func
	for _, f := range a.Prog.Funcs {
		if f.Ent != 0 && a.Res.Entity(f.Ent).Name == "walk" {
			walk = f
		}
	}
	box := -1
	for i, l := range walk.Locals {
		if l.Name == "box" && l.Ent != 0 {
			box = i
		}
	}
	if box < 0 || walk.Slot[box] < 0 || !walk.IsDirect(midir.LocalID(box)) || a.Prog.SlotBytes(walk, midir.LocalID(box)) != 32 {
		t.Fatalf("box should sit in the frame as 32 inline bytes")
	}
	// kgframe's header is desc, awaited, state, nargs and args: 32 bytes before slots[].
	slots := int64(0)
	for i := range walk.Locals {
		if walk.Slot[i] >= 0 {
			slots += a.Prog.SlotBytes(walk, midir.LocalID(i))
		}
	}
	want := fmt.Sprintf("walk: async frame, %d bytes", 32+slots+8*int64(len(walk.Params)))
	if !slices.Contains(report, want) {
		t.Fatalf("report %v lacks %q", report, want)
	}
}
