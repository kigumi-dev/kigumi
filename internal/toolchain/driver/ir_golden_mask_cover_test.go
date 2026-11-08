package driver_test

import (
	"path/filepath"
	"strings"
	"testing"
)

// A layout or ABI change the program's own code depends on must move the pinned IR, even outside any
// function body.
func TestPinnedIRSeesLayoutAndSignatureChanges(t *testing.T) {
	irs := map[string]string{}
	for _, m := range []struct{ fixture, name, old, new string }{
		{"f32_native_c_call", "record layout", `%"kg.agg.golden.Pair" = type { float, float }`, `%"kg.agg.golden.Pair" = type { double, float }`},
		{"text_parse_whitespace", "Option layout", `%"kg.adt.Option[Int]" = type { i32, [1 x i64] }`, `%"kg.adt.Option[Int]" = type { i8, [1 x i32] }`},
		{"f32_native_c_call", "extern signature", `declare float @sqrtf(float)`, `declare double @sqrtf(double)`},
		{"f32_native_c_call", "type descriptor", `i32 0, i32 2, ptr @".type.golden.Pair.fields"`, `i32 0, i32 3, ptr @".type.golden.Pair.fields"`},
		{"f32_native_c_call", "type table", `@rt_types = global [3 x ptr]`, `@rt_types = global [2 x ptr]`},
		{"f32_native_c_call", "runtime record", `%rt_variant = type { ptr, ptr, i32, i32, [64 x i8] }`, `%rt_variant = type { ptr, ptr, i32, i32, [32 x i8] }`},
		{"f32_native_c_call", "body", `fmul float`, `fadd float`},
	} {
		ir, ok := irs[m.fixture]
		if !ok {
			ir = compileGolden(t, filepath.Join("../../../testdata/run", m.fixture+".txtar"))
			irs[m.fixture] = ir
		}
		if !strings.Contains(ir, m.old) {
			t.Fatalf("%s: the IR of %s no longer contains %q", m.name, m.fixture, m.old)
		}
		if pinnedIR(strings.Replace(ir, m.old, m.new, 1), irGoldenModule) == pinnedIR(ir, irGoldenModule) {
			t.Errorf("%s: pinnedIR hides the change %q -> %q", m.name, m.old, m.new)
		}
	}
}
