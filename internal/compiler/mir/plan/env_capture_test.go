package plan_test

import (
	"os"
	"strings"
	"testing"

	mirbuild "kigumi/internal/compiler/mir/build"
	mirplan "kigumi/internal/compiler/mir/plan"
	"kigumi/internal/compiler/sem"
	devfixture "kigumi/internal/devkit/fixture"
	"kigumi/internal/devkit/mirfixture"
)

// runtimeTypeFixtures are the run fixtures whose lambdas read runtime types.
var runtimeTypeFixtures = []string{
	"simd_descriptor_method",
	"simd_witness_shape",
	"simd_descriptor_drop",
	"const_generic_drop",
	"error_as_generic_instance",
	"type_test_generic_instance",
	"type_test_nested",
	"error_as_erased_values",
	"const_generic_drop_values",
	"async_generic_box_error",
	"type_test_closure_instance",
	"iface_std_const_method",
	"type_test_fn_value_instance",
	"async_generic_fn_value_type_test",
	"type_test_witness_fn_value",
	"comptime_generic_type_test",
	"type_test_poly_recursion_method",
	"type_test_poly_recursion_vtable",
	"type_test_wildcard_forward",
	"type_test_instantiation_budget",
	"drop_runtime_instance",
	"async_runtime_type_args",
	"type_test_runtime_lambda",
	"const_generic_wide_identity",
	"type_test_opened_box_fn_value",
	"type_test_opened_value_uses",
	"type_test_runtime_fn_values",
	"type_test_runtime_async_value",
	"type_test_runtime_drop_value",
	"type_test_runtime_comptime_value",
	"type_test_many_params",
	"type_test_opens_through_calls",
	"type_test_deep_recursion",
	"type_test_const_open",
	"error_as_many_types",
	"type_test_opened_closure_identity",
	"type_test_lambda_runtime_types",
	"async_lambda_runtime_types",
	"type_test_returned_lambda",
	"type_test_lambda_in_values",
	"type_test_lambda_box_identity",
	"type_test_lambda_generic_helper",
	"type_test_lambda_method_value",
	"type_test_method_value_own_params",
}

// TestEnvCapturesWhatSemCharged holds the environment pass to sem's prediction: a lambda captures a runtime type
// exactly when sem charged it for one, so no allocation appears that sem's effect summaries missed.
func TestEnvCapturesWhatSemCharged(t *testing.T) {
	for _, name := range runtimeTypeFixtures {
		t.Run(name, func(t *testing.T) {
			path, err := devfixture.Find("../../../..", "run/"+name)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			src := string(data)
			src = src[strings.Index(src, "-- src --\n")+len("-- src --\n"):]
			src = src[:strings.Index(src, "\n-- ")+1]
			p := mirbuild.Program(mirfixture.CheckSourceMir(t, src))
			if err := mirplan.Apply(p, mirplan.Transform{Name: "plan-rc", Run: mirplan.RC}, mirplan.Transform{Name: "specialize", Run: mirplan.Specialize}); err != nil {
				t.Fatal(err)
			}
			captured := map[sem.EntityID]bool{}
			for _, f := range p.Funcs {
				if f.Ent == 0 || p.R.Entity(f.Ent).Kind != sem.EntClosure {
					continue
				}
				origin := p.R.ClosureOrigin(f.Ent)
				captured[origin] = captured[origin] || len(p.R.Closure(f.Ent).Captures) > len(p.R.Closure(origin).Captures)
			}
			for origin, got := range captured {
				if charged := p.R.CapturesRuntimeTypes(origin); got != charged {
					t.Errorf("%s captures a runtime type: %v, sem charged it: %v", p.FuncName(origin), got, charged)
				}
			}
		})
	}
}
