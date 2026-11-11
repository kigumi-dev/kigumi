package vm_test

import (
	"bytes"
	devfixture "kigumi/internal/devkit/fixture"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kigumi/internal/backend/vm"
	"kigumi/internal/compiler/mir"
	mirbuild "kigumi/internal/compiler/mir/build"
	mirplan "kigumi/internal/compiler/mir/plan"
)

// quiesce lets goroutines an earlier test left running finish, since they
// allocate and free into the same counters.
func quiesce() {
	for prev, i := -1, 0; i < 100; i++ {
		n := runtime.NumGoroutine()
		if n == prev {
			return
		}
		prev = n
		time.Sleep(50 * time.Millisecond)
	}
}

// liveGate keeps comptime folding out of a tracking window: the fold takes the write side with counting paused, runs hold the read side.
type liveGate struct {
	mu sync.RWMutex
	on bool
}

func (g *liveGate) fold(prog *mir.Program) []vm.ComptimeError {
	if g == nil || len(prog.Comptime) == 0 {
		return vm.FoldComptime(prog)
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	vm.PauseLive(false)
	defer vm.PauseLive(g.on)
	return vm.FoldComptime(prog)
}

func (g *liveGate) run(t *testing.T, prog *mir.Program) (string, int, error) {
	if g != nil {
		g.mu.RLock()
		defer g.mu.RUnlock()
	}
	return runUncounted(t, prog)
}

// runUncounted is runCounted without the live accounting, safe to call from parallel subtests.
func runUncounted(t *testing.T, prog *mir.Program) (string, int, error) {
	var out, errOut bytes.Buffer
	atExit, reached := "", false
	code, err := vm.RunReleasing(prog, vm.Options{Cwd: t.TempDir(), Args: []string{"main", "one", "two"}}, &out, &errOut, func() { atExit, reached = out.String(), true })
	if !reached {
		atExit = out.String()
	}
	return atExit, code, err
}

// preparedProg checks, builds and plans a run fixture for the VM, skipping the test when it cannot run there.
func preparedProg(t *testing.T, fx fixture, gate *liveGate, unsupported *atomic.Int64) *mir.Program {
	res := checkSource(t, fx.src)
	if res.HasErrors() {
		t.Skip("check errors")
	}
	prog := mirbuild.Program(res)
	if prog.HasErrors() {
		t.Skip("ownership errors")
	}
	plans := []mirplan.Transform{{Name: "plan-rc", Run: mirplan.RC}, {Name: "specialize", Run: mirplan.Specialize}, {Name: "elide-rc", Run: mirplan.ElideRC}, {Name: "plan-direct", Run: mirplan.PlanDirect}}
	if err := mirplan.Apply(prog, plans...); err != nil {
		t.Fatalf("plans: %v", err)
	}
	if errs := gate.fold(prog); len(errs) > 0 {
		t.Fatalf("comptime: %v", errs[0].Err)
	}
	if ok, why := vm.Supports(prog); !ok {
		unsupported.Add(1)
		t.Skip(why)
	}
	return prog
}

func corpusFixtures(t *testing.T) (zero, nonzero []string) {
	files, err := devfixture.Corpus("../../..", "run")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no run fixtures")
	}
	for _, path := range devfixture.ShardPaths(files) {
		if readFixture(t, path).exit == 0 {
			zero = append(zero, path)
		} else {
			nonzero = append(nonzero, path)
		}
	}
	return zero, nonzero
}

func runCorpusFixture(t *testing.T, path string, gate *liveGate, unsupported *atomic.Int64) {
	fx := readFixture(t, path)
	if !strings.Contains(fx.src, "/proc/self/fd") {
		t.Parallel()
	}
	out, code, err := gate.run(t, preparedProg(t, fx, gate, unsupported))
	if err != nil {
		t.Fatalf("vm: %v", err)
	}
	if strings.Contains(out, "<value>") {
		t.Errorf("stdout contains the runtime's unknown-value placeholder")
	}
	if out != fx.stdout || code != fx.exit {
		t.Errorf("mismatch (exit %d, want %d)\n--- got ---\n%s--- want ---\n%s", code, fx.exit, out, fx.stdout)
	}
}

// The live counters are process-wide, so exit-0 fixtures share one window; a
// free of an object made outside it is counted apart. The exit-0 corpus must
// leave no object live: there is no baseline allowance to hide a leak behind.
// This file sorts first so no earlier test leaves goroutines allocating into
// the live counters.
func TestRunCorpusOnVM(t *testing.T) {
	zero, _ := corpusFixtures(t)
	var unsupported atomic.Int64
	quiesce()
	gate := &liveGate{on: true}
	vm.TrackLive(true)
	t.Cleanup(func() {
		quiesce()
		total, strays, freedUses := vm.LiveObjects(), vm.LiveStrays(), vm.LiveFreedUses()
		vm.TrackLive(false)
		t.Logf("%d fixtures skipped as unsupported", unsupported.Load())
		if total != 0 {
			t.Errorf("%d objects live after the exit-0 fixtures, want 0", total)
		}
		if strays != 0 {
			t.Errorf("%d objects freed that were made outside their run", strays)
		}
		if freedUses != 0 {
			t.Errorf("%d releases reached an object that was already freed", freedUses)
		}
	})
	for _, path := range zero {
		t.Run(filepath.Base(path), func(t *testing.T) { runCorpusFixture(t, path, gate, &unsupported) })
	}
}

// TestRunCorpusOnVMExitCodes runs the fixtures that exit nonzero, which leave objects live by design and so are never counted.
func TestRunCorpusOnVMExitCodes(t *testing.T) {
	_, nonzero := corpusFixtures(t)
	var unsupported atomic.Int64
	for _, path := range nonzero {
		t.Run(filepath.Base(path), func(t *testing.T) { runCorpusFixture(t, path, nil, &unsupported) })
	}
}
