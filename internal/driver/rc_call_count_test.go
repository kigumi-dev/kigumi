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
	"fixtures":         601,
	"rt_retain":        4173,
	"rt_release":       42985,
	"rt_release_outer": 13348,
	"rt_drop":          25353,
	"rt_cell_init":     2692,
	"rt_cell_take":     1119,
	"rt_setfield_swap": 174,
	"rt_setindex_swap": 35,
}

// The same call sites over the fixtures that existed before the retain / release elimination and the lent receivers,
// measured on the unchanged compiler; the totals above for those fixtures must stay at or below these.
var baselineRCCalls = map[string]int{
	"fixtures":         528,
	"rt_retain":        4911,
	"rt_release":       51119,
	"rt_release_outer": 12954,
	"rt_drop":          25759,
	"rt_cell_init":     2455,
	"rt_cell_take":     2503,
	"rt_setfield_swap": 203,
	"rt_setindex_swap": 32,
}

var addedAfterBaseline = map[string]bool{
	"f32_native_register.txtar":                     true,
	"f32_native_containers.txtar":                   true,
	"f32_native_c_call.txtar":                       true,
	"mut_self_lent_receiver.txtar":                  true,
	"ffi_sizeof_fixed_array.txtar":                  true,
	"untyped_branch_operand.txtar":                  true,
	"range_loop_scalar.txtar":                       true,
	"range_loop_wide.txtar":                         true,
	"shift_context.txtar":                           true,
	"noalloc_lend_calls.txtar":                      true,
	"defer_reads_latest.txtar":                      true,
	"defer_reserved_paths.txtar":                    true,
	"defer_second_binding.txtar":                    true,
	"exclusive_access_accepted.txtar":               true,
	"const_qualified_package.txtar":                 true,
	"pattern_move_variant.txtar":                    true,
	"defer_reserved_helpers.txtar":                  true,
	"closure_once.txtar":                            true,
	"method_value_owned_places_panic.txtar":         true,
	"method_value_owned_places.txtar":               true,
	"escaping_stash_variadic.txtar":                 true,
	"escaping_param_keep.txtar":                     true,
	"os_string_host.txtar":                          true,
	"os_string_values.txtar":                        true,
	"float_ieee_operators.txtar":                    true,
	"defer_reserved_reads.txtar":                    true,
	"defer_reserved_mut.txtar":                      true,
	"method_value_generic_receiver.txtar":           true,
	"async_task_panic_method_value_map.txtar":       true,
	"async_task_panic_direct_try_map_capture.txtar": true,
	"view_ref_store.txtar":                          true,
	"view_map_store.txtar":                          true,
	"view_infer_fold.txtar":                         true,
	"view_holder_reassign.txtar":                    true,
	"view_array_store_read.txtar":                   true,
	"view_array_containers.txtar":                   true,
	"async_task_panic_map_partial_order.txtar":      true,
	"async_root_panic_map_keeps_frames.txtar":       true,
	"async_root_panic_filter_keeps_frames.txtar":    true,
	"defer_reserved_patterns.txtar":                 true,
	"result_paren_tail.txtar":                       true,
	"defer_reserved_fields.txtar":                   true,
	"async_task_panic_temps_callback.txtar":         true,
	"async_task_panic_lent_receiver.txtar":          true,
	"async_task_panic_fold_owned_items.txtar":       true,
	"async_task_panic_callback_lent.txtar":          true,
	"async_task_panic_callback_capture.txtar":       true,
	"async_task_drop_panic_callback_args.txtar":     true,
	"isr_notrap.txtar":                              true,
	"exclusive_access_places.txtar":                 true,
	"async_cancel_defer_positions.txtar":            true,
	"async_cancel_defer_panic.txtar":                true,
	"async_cancel_defer_observed.txtar":             true,
	"async_cancel_defer_stages.txtar":               true,
	"async_cancel_defer.txtar":                      true,
	"defer_fail_return.txtar":                       true,
	"defer_fail_mixed.txtar":                        true,
	"defer_fail_compound_exit.txtar":                true,
	"scalar_box_values.txtar":                       true,
	"callback_contract_with_address.txtar":          true,
	"callback_contract_shared.txtar":                true,
	"callback_contract_ffi.txtar":                   true,
	"callback_contract_box.txtar":                   true,
	"callback_contract_array.txtar":                 true,
	"async_task_panic_try_map.txtar":                true,
	"panic_int_mod_zero.txtar":                      true,
	"panic_int_div_zero.txtar":                      true,
	"panic_array_index_read.txtar":                  true,
	"shell_interp_argv.txtar":                       true,
	"float_map_key.txtar":                           true,
	"to_float_callback.txtar":                       true,
	"method_value_owned_receiver.txtar":             true,
	"async_task_panic_owned_method_value.txtar":     true,
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
				m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../std", Entry: "main/main.kg"})
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
