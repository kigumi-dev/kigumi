package driver_test

import (
	"bytes"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/mir"
)

// Lowering only what the module reaches must leave the reachable set and the emitted IR exactly as lowering every body does.
func TestPrunedProgramMatchesWhole(t *testing.T) {
	fixtures := append(slices.Clone(mutantBases), "comptime_multi_block", "fix_crt_shell_pipe_chain", "ffi_with_address_callback",
		"async_direct_frame_panic", "direct_protocol_values", "isr", "conform_generic_method_param_iface_dispatch", "json_map_keys", "live_temp_release")
	for _, name := range fixtures {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			pruned, prunedIR := prunedOrWhole(t, name, false)
			whole, wholeIR := prunedOrWhole(t, name, true)
			if pruned != whole {
				t.Errorf("reachability differs\n--- pruned ---\n%s--- whole ---\n%s", pruned, whole)
			}
			if prunedIR != wholeIR {
				t.Error("emitted IR differs")
			}
		})
	}
}

func prunedOrWhole(t *testing.T, name string, whole bool) (reach, ir string) {
	load := func() *driver.Module {
		root, _, _, _ := loadRunFixture(t, "../../testdata/run/"+name+".txtar")
		m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../std", Entry: "main/main.kg"})
		if err != nil {
			t.Fatal(err)
		}
		if whole {
			driver.LowerAll(m)
		}
		return m
	}
	a, err := driver.Analyze(load())
	if err != nil || a.Res.HasErrors() {
		t.Fatalf("analyze: %v", err)
	}
	rs := mir.Reachable(a.Prog)
	var sites strings.Builder
	for _, op := range rs.Ops {
		for _, s := range rs.AllOpSites[op] {
			fmt.Fprintf(&sites, " %d@%s:%d", op, a.Prog.ByEnt[s.Caller].Name, s.Node)
		}
	}
	// Instances are minted in the order specialization meets them, so they are compared by name.
	reach = fmt.Sprintf("%v\nops %v\nsites%s\ntypes %v\n", rs.Report(a.Res), rs.Ops, sites.String(), slices.Sorted(maps.Keys(rs.Types)))
	var stderr bytes.Buffer
	ir, ok, err := driver.Compile(load(), &stderr)
	if err != nil || !ok {
		t.Fatalf("compile: %v\n%s", err, stderr.String())
	}
	return reach, ir
}
