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
	n := vm.LiveObjects()
	settled := runtime.NumGoroutine() <= base
	vm.TrackLive(false)
	if !reached {
		atExit = out.String()
	}
	if err != nil || code != 0 || !settled {
		return atExit, code, err
	}
	liveRan[name] = true
	if n != 0 {
		liveGot[name] = n
	}
	if n > known[name] && os.Getenv("UPDATE") == "" {
		t.Errorf("%d objects live at exit, %d allowed", n, known[name])
	}
	return atExit, code, nil
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

// This file sorts first so no earlier test leaves goroutines allocating into the live counters.
func TestRunCorpusOnVM(t *testing.T) {
	files, _ := filepath.Glob("../../testdata/run/*.txtar")
	skipped := 0
	known := loadLive(t)
	quiesce()
	for _, path := range files {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			fx := readFixture(t, path)
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
			if errs := vm.FoldComptime(prog); len(errs) > 0 {
				t.Fatalf("comptime: %v", errs[0].Err)
			}
			if ok, why := vm.Supports(prog); !ok {
				skipped++
				t.Skip(why)
			}
			out, code, err := runCounted(t, name, prog, known)
			if err != nil {
				t.Fatalf("vm: %v", err)
			}
			if out != fx.stdout || code != fx.exit {
				t.Errorf("mismatch (exit %d, want %d)\n--- got ---\n%s--- want ---\n%s", code, fx.exit, out, fx.stdout)
			}
		})
	}
	finishLive(t, known)
	t.Logf("%d fixtures skipped as unsupported", skipped)
}
