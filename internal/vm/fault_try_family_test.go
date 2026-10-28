package vm_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/vm"
)

// A refused allocation inside a try* call returns Err, and the Err run frees everything the unfaulted run does: the same live-object count at exit.
// Runs share the global live counters, so they go one after another.
func TestFaultInjectionTryFamilyLeavesNothingLive(t *testing.T) {
	files, _ := filepath.Glob("../../testdata/oom/*.kg")
	if len(files) < 20 {
		t.Fatalf("found %d programs", len(files))
	}
	quiesce()
	for _, path := range files {
		name := strings.TrimSuffix(filepath.Base(path), ".kg")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			res := checkSource(t, string(src))
			if res.HasErrors() {
				t.Fatal(res.Render())
			}
			prog := mir.Build(res)
			plans := []mir.Transform{{Name: "plan-rc", Run: mir.PlanRC}, {Name: "specialize", Run: mir.Specialize}, {Name: "elide-rc", Run: mir.ElideRC}, {Name: "plan-direct", Run: mir.PlanDirect}}
			if err := prog.Apply(plans...); err != nil {
				t.Fatal(err)
			}
			run := func(failAt int64) (stdout string, live int64) {
				var out, errOut bytes.Buffer
				vm.TrackLive(true)
				defer vm.TrackLive(false)
				if _, err := vm.RunReleasing(prog, vm.Options{FaultInjectAt: failAt, FaultArmOnEprint: true, Cwd: t.TempDir()}, &out, &errOut, func() {}); err != nil {
					t.Fatalf("N=%d: %v", failAt, err)
				}
				return out.String(), vm.LiveObjects()
			}
			baseOut, baseLive := run(0)
			if baseOut != "" {
				t.Fatalf("the unfaulted run printed %q", baseOut)
			}
			sawErr := false
			for n := int64(1); n <= 40; n++ {
				out, live := run(n)
				if out != "err\n" {
					continue
				}
				sawErr = true
				if live != baseLive {
					t.Errorf("N=%d: %d objects live after the Err return, %d without a fault", n, live, baseLive)
				}
			}
			if !sawErr && !tryFamilyNoAlloc[name] {
				t.Error("no fault tick produced an Err")
			}
		})
	}
}

// tryFamilyNoAlloc names the fixtures whose success path allocates nothing on the VM, so no tick can refuse.
var tryFamilyNoAlloc = map[string]bool{"map_set_existing_key": true, "map_set_existing_key_twice": true, "alloc_wrap": true, "alloc_wrap_twice": true, "shared_get": true, "shared_get_twice": true}
