package vm_test

import (
	"io"
	"testing"
	"time"

	"kigumi/internal/mir"
	"kigumi/internal/vm"
)

func TestSandboxStepBudgetStopsAnEmptyLoop(t *testing.T) {
	prog := mir.Build(checkSource(t, "print \"x\"\n"))
	if err := prog.Apply(mir.Transform{Name: "plan-rc", Run: mir.PlanRC}); err != nil {
		t.Fatal(err)
	}
	entry := prog.Funcs[len(prog.Funcs)-1]
	for _, f := range prog.Funcs {
		if f.Name == "main" {
			entry = f
		}
	}
	entry.Blocks = []mir.Block{{Term: mir.Term{Op: mir.TermJump, Targets: []mir.BlockID{0}}}}
	done := make(chan int, 1)
	go func() {
		code, _ := vm.Run(prog, vm.Options{Sandbox: true, StepLimit: 1000, DepthLimit: 100}, io.Discard, io.Discard)
		done <- code
	}()
	select {
	case code := <-done:
		if code == 0 {
			t.Fatal("an empty loop finished")
		}
	case <-time.After(20 * time.Second):
		t.Fatal("an empty loop ran past the step limit")
	}
}
