package vm_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"kigumi/internal/mir"
	"kigumi/internal/vm"
)

const liveFile = "../../testdata/vm_live.txt"

var (
	liveGot = map[string]int64{}
	liveRan = map[string]bool{}
)

func loadLive(t *testing.T) map[string]int64 {
	data, err := os.ReadFile(liveFile)
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]int64{}
	for _, l := range strings.Split(string(data), "\n") {
		name, n, ok := strings.Cut(l, " ")
		if !ok {
			continue
		}
		v, err := strconv.ParseInt(n, 10, 64)
		if err != nil {
			t.Fatalf("%s: %q", liveFile, l)
		}
		known[name] = v
	}
	return known
}

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

// runCounted counts the objects prog leaves live at exit; runs that fail or leave workers behind are not counted.
func runCounted(t *testing.T, name string, prog *mir.Program, known map[string]int64) (string, int, error) {
	var out, errOut bytes.Buffer
	atExit, reached := "", false
	base := runtime.NumGoroutine()
	vm.TrackLive(true)
	code, err := vm.RunReleasing(prog, vm.Options{Cwd: t.TempDir(), Args: []string{"main", "one", "two"}}, &out, &errOut, func() { atExit, reached = out.String(), true })
	for i := 0; i < 300 && runtime.NumGoroutine() > base; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	n, strays := vm.LiveObjects(), vm.LiveStrays()
	settled := runtime.NumGoroutine() <= base
	vm.TrackLive(false)
	if !reached {
		atExit = out.String()
	}
	if err != nil || code != 0 || !settled {
		return atExit, code, err
	}
	liveRan[name] = true
	if strays != 0 {
		t.Errorf("%s: %d objects freed that were made outside the run", name, strays)
	}
	if n != 0 {
		liveGot[name] = n
	}
	if n > known[name] && os.Getenv("UPDATE") == "" {
		t.Errorf("%s: %d objects live at exit, %d allowed", name, n, known[name])
	}
	return atExit, code, nil
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

func finishLive(t *testing.T, known map[string]int64) {
	if os.Getenv("UPDATE") != "" {
		var names []string
		for name := range liveGot {
			names = append(names, name)
		}
		sort.Strings(names)
		var sb strings.Builder
		for _, name := range names {
			fmt.Fprintf(&sb, "%s %d\n", name, liveGot[name])
		}
		os.WriteFile(liveFile, []byte(sb.String()), 0o644)
		return
	}
	for name, n := range known {
		if liveRan[name] && liveGot[name] < n {
			t.Errorf("%s: %d objects live at exit, below the %d listed (UPDATE=1 to tighten)", name, liveGot[name], n)
		}
	}
}

// preparedProg checks, builds and plans a run fixture for the VM, skipping the test when it cannot run there.
func preparedProg(t *testing.T, fx fixture, gate *liveGate, unsupported *atomic.Int64) *mir.Program {
	res := checkSource(t, fx.src)
	if res.HasErrors() {
		t.Skip("check errors")
	}
	prog := mir.Build(res)
	if prog.HasErrors() {
		t.Skip("ownership errors")
	}
	plans := []mir.Transform{{Name: "plan-rc", Run: mir.PlanRC}, {Name: "specialize", Run: mir.Specialize}, {Name: "plan-direct", Run: mir.PlanDirect}}
	if err := prog.Apply(plans...); err != nil {
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
	files, _ := filepath.Glob("../../testdata/run/*.txtar")
	for _, path := range files {
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
	if out != fx.stdout || code != fx.exit {
		t.Errorf("mismatch (exit %d, want %d)\n--- got ---\n%s--- want ---\n%s", code, fx.exit, out, fx.stdout)
	}
}

// liveSettled is true once TestRunCorpusOnVM saw the exit-0 fixtures leave nothing live and free no stray, so the per-fixture pass has nothing to attribute.
var liveSettled bool

// The live counters are process-wide, so exit-0 fixtures share one window; a free of an object made outside it is counted apart.
// This file sorts first so no earlier test leaves goroutines allocating into the live counters.
func TestRunCorpusOnVM(t *testing.T) {
	zero, _ := corpusFixtures(t)
	var unsupported atomic.Int64
	known := loadLive(t)
	quiesce()
	gate := &liveGate{on: true}
	vm.TrackLive(true)
	t.Cleanup(func() {
		quiesce()
		total, strays, freedUses := vm.LiveObjects(), vm.LiveStrays(), vm.LiveFreedUses()
		vm.TrackLive(false)
		t.Logf("%d fixtures skipped as unsupported", unsupported.Load())
		var allowed int64
		for _, n := range known {
			allowed += n
		}
		if total > allowed && os.Getenv("UPDATE") == "" {
			t.Errorf("%d objects live after the exit-0 fixtures, %d allowed (TestRunCorpusOnVMLive names the fixtures)", total, allowed)
		}
		if strays != 0 {
			t.Errorf("%d objects freed that were made outside their run (TestRunCorpusOnVMLive names the fixtures)", strays)
		}
		if freedUses != 0 {
			t.Errorf("%d releases reached an object that was already freed", freedUses)
		}
		liveSettled = total == 0 && strays == 0 && len(known) == 0 && os.Getenv("UPDATE") == ""
	})
	for _, path := range zero {
		t.Run(filepath.Base(path), func(t *testing.T) { runCorpusFixture(t, path, gate, &unsupported) })
	}
}

// TestRunCorpusOnVMLive counts each exit-0 fixture alone when the combined total is nonzero, a fixture lists an allowance or UPDATE rewrites the list.
func TestRunCorpusOnVMLive(t *testing.T) {
	if liveSettled {
		t.Skip("no fixture leaves objects live")
	}
	zero, _ := corpusFixtures(t)
	known := loadLive(t)
	quiesce()
	for _, path := range zero {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			prog := preparedProg(t, readFixture(t, path), nil, new(atomic.Int64))
			if _, _, err := runCounted(t, name, prog, known); err != nil {
				t.Fatalf("vm: %v", err)
			}
		})
	}
	finishLive(t, known)
}

// TestRunCorpusOnVMExitCodes runs the fixtures that exit nonzero, which leave objects live by design and so are never counted.
func TestRunCorpusOnVMExitCodes(t *testing.T) {
	_, nonzero := corpusFixtures(t)
	var unsupported atomic.Int64
	for _, path := range nonzero {
		t.Run(filepath.Base(path), func(t *testing.T) { runCorpusFixture(t, path, nil, &unsupported) })
	}
}
