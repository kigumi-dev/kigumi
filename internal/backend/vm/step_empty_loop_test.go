package vm_test

import (
	"io"
	"strings"
	"testing"
	"time"

	"kigumi/internal/backend/vm"
	mirbuild "kigumi/internal/compiler/mir/build"
	midir "kigumi/internal/compiler/mir/ir"
	mirplan "kigumi/internal/compiler/mir/plan"
)

func TestSandboxStepBudgetStopsAnEmptyLoop(t *testing.T) {
	prog := mirbuild.Program(checkSource(t, "print \"x\"\n"))
	if err := mirplan.Apply(prog, mirplan.Transform{Name: "plan-rc", Run: mirplan.RC}); err != nil {
		t.Fatal(err)
	}
	entry := prog.Funcs[len(prog.Funcs)-1]
	for _, f := range prog.Funcs {
		if f.Name == "main" {
			entry = f
		}
	}
	entry.Blocks = []midir.Block{{Term: midir.Term{Op: midir.TermJump, Targets: []midir.BlockID{0}}}}
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

func TestSandboxStepBudgetStopsAnEmptyAsyncLoop(t *testing.T) {
	src := "import task from std/task\n\nasync fn spin() -> Int {\n    1\n}\n\nlet ex = task.currentThreadExecutor()\nprint \"${ex.runToCompletion(spin())}\"\n"
	prog := mirbuild.Program(checkSource(t, src))
	if err := mirplan.Apply(prog, mirplan.Transform{Name: "plan-rc", Run: mirplan.RC}); err != nil {
		t.Fatal(err)
	}
	spun := false
	for _, f := range prog.Funcs {
		if f.Name == "spin" || strings.HasSuffix(f.Name, ".spin") {
			f.Blocks = []midir.Block{{Term: midir.Term{Op: midir.TermJump, Targets: []midir.BlockID{0}}}}
			spun = true
		}
	}
	if !spun {
		t.Fatal("no async fn spin in the program")
	}
	done := make(chan int, 1)
	go func() {
		code, _ := vm.Run(prog, vm.Options{Sandbox: true, StepLimit: 1000, DepthLimit: 100}, io.Discard, io.Discard)
		done <- code
	}()
	select {
	case code := <-done:
		if code == 0 {
			t.Fatal("an empty async loop finished")
		}
	case <-time.After(20 * time.Second):
		t.Fatal("an empty async loop ran past the step limit")
	}
}
