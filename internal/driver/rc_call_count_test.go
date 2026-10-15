package driver_test

import (
	"bytes"
	"path/filepath"
	"regexp"
	"sync"
	"testing"

	"kigumi/internal/driver"
)

var rcCallSites = map[string]*regexp.Regexp{
	"rt_retain":        regexp.MustCompile(`call [^@\n]*@rt_retain\(`),
	"rt_release":       regexp.MustCompile(`call [^@\n]*@rt_release\(`),
	"rt_cell_init":     regexp.MustCompile(`call [^@\n]*@rt_cell_init\(`),
	"rt_cell_take":     regexp.MustCompile(`call [^@\n]*@rt_cell_take\(`),
	"rt_setfield_swap": regexp.MustCompile(`call [^@\n]*@rt_setfield_swap\(`),
	"rt_setindex_swap": regexp.MustCompile(`call [^@\n]*@rt_setindex_swap\(`),
	"rt_drop":          regexp.MustCompile(`call [^@\n]*@rt_drop\(`),
	"rt_release_outer": regexp.MustCompile(`call [^@\n]*@rt_release_outer\(`),
}

// Call sites of the count and cell runtime calls in the IR of every native run fixture, std included.
var pinnedRCCalls = map[string]int{
	"fixtures":         537,
	"rt_retain":        3571,
	"rt_release":       40876,
	"rt_release_outer": 11378,
	"rt_drop":          24092,
	"rt_cell_init":     2495,
	"rt_cell_take":     1056,
	"rt_setfield_swap": 129,
	"rt_setindex_swap": 33,
}

// The same call sites over the fixtures that existed before the retain / release elimination and the lent receivers,
// measured on the unchanged compiler; the totals above for those fixtures must stay at or below these.
var baselineRCCalls = map[string]int{
	"fixtures":         529,
	"rt_retain":        4847,
	"rt_release":       51713,
	"rt_release_outer": 12928,
	"rt_drop":          26695,
	"rt_cell_init":     2439,
	"rt_cell_take":     2487,
	"rt_setfield_swap": 197,
	"rt_setindex_swap": 28,
}

var addedAfterBaseline = map[string]bool{
	"f32_native_register.txtar":    true,
	"f32_native_containers.txtar":  true,
	"f32_native_c_call.txtar":      true,
	"mut_self_lent_receiver.txtar": true,
	"ffi_sizeof_fixed_array.txtar": true,
	"untyped_branch_operand.txtar": true,
	"range_loop_scalar.txtar":      true,
	"noalloc_lend_calls.txtar":     true,
}

func TestRCCallCounts(t *testing.T) {
	manifest := loadManifest(t)
	files, _ := filepath.Glob("../../testdata/run/*.txtar")
	var mu sync.Mutex
	totals := map[string]int{}
	baseline := map[string]int{}
	compiled := 0
	t.Run("corpus", func(t *testing.T) {
		for _, path := range files {
			fx := manifest.ByID(fixtureID(path))
			if fx == nil || !fx.Runs("native") {
				continue
			}
			t.Run(filepath.Base(path), func(t *testing.T) {
				t.Parallel()
				root, _, _, _ := loadRunFixture(t, path)
				m, err := driver.LoadModule(root, driver.LoadOptions{StdRoot: "../../std", Entry: "main/main.kg"})
				if err != nil {
					t.Fatal(err)
				}
				var stderr bytes.Buffer
				ir, ok, err := driver.Compile(m, &stderr)
				if err != nil || !ok {
					t.Skipf("no IR: %v %s", err, stderr.String())
				}
				mu.Lock()
				defer mu.Unlock()
				compiled++
				for name, re := range rcCallSites {
					n := len(re.FindAllStringIndex(ir, -1))
					totals[name] += n
					if !addedAfterBaseline[filepath.Base(path)] {
						baseline[name] += n
					}
				}
			})
		}
	})
	totals["fixtures"] = compiled
	baseline["fixtures"] = compiled - len(addedAfterBaseline)
	t.Logf("totals: %v", totals)
	for name, want := range pinnedRCCalls {
		if totals[name] != want {
			t.Errorf("%s call sites over the run corpus = %d, pinned %d (re-pin pinnedRCCalls when the corpus or std changes)", name, totals[name], want)
		}
	}
	for name, was := range baselineRCCalls {
		if name != "fixtures" && baseline[name] > was {
			t.Errorf("%s call sites over the pre-existing fixtures = %d, above the unchanged compiler's %d", name, baseline[name], was)
		}
	}
	if baseline["fixtures"] != baselineRCCalls["fixtures"] {
		t.Errorf("pre-existing fixtures = %d, baseline measured over %d", baseline["fixtures"], baselineRCCalls["fixtures"])
	}
	if baseline["rt_retain"] >= baselineRCCalls["rt_retain"] || baseline["rt_release"] >= baselineRCCalls["rt_release"] {
		t.Errorf("retain / release no longer drop below the unchanged compiler's %d / %d", baselineRCCalls["rt_retain"], baselineRCCalls["rt_release"])
	}
}
