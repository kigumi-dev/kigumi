package driver_test

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

func fixtureIR(t *testing.T, path string) string {
	t.Helper()
	root, _, _, _ := loadRunFixture(t, path)
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: "../../../std", Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	ir, ok, err := driver.Compile(m, &stderr)
	if err != nil || !ok {
		t.Fatalf("compile: %v\n%s", err, stderr.String())
	}
	return ir
}

func functionBody(t *testing.T, ir, name string) string {
	t.Helper()
	re := regexp.MustCompile(`(?s)define [^\n]*@"[^"]*` + regexp.QuoteMeta(name) + `"\(.*?\n\}\n`)
	body := re.FindString(ir)
	if body == "" {
		t.Fatalf("no function %s in the IR", name)
	}
	return body
}

// An f32 is an LLVM float in locals, parameters, results and direct record fields, not a double rounded after each operation.
func TestF32NativeRegisters(t *testing.T) {
	ir := fixtureIR(t, "../../../testdata/run/f32_native_register.txtar")
	mix := functionBody(t, ir, "main.mix")
	if !strings.Contains(mix, "(float %p0, float %p1)") || !strings.HasPrefix(mix, "define float ") {
		t.Errorf("mix does not take and return float:\n%s", mix)
	}
	for _, want := range []string{"fmul float", "fdiv float", "fadd float", "fsub float"} {
		if !strings.Contains(mix, want) {
			t.Errorf("mix lacks %q", want)
		}
	}
	for _, banned := range []string{"@rt_float", "@rt_binop", "double"} {
		if strings.Contains(mix, banned) {
			t.Errorf("mix still goes through %q:\n%s", banned, mix)
		}
	}
	if !regexp.MustCompile(`%"kg\.agg\.[^"]*main\.Vec" = type \{ float, float \}`).MatchString(ir) {
		t.Error("a record of two f32 fields is not a { float, float } aggregate")
	}
	if !regexp.MustCompile(`%"kg\.arr\.[^"]*FixedArray\[f32,3\]" = type \{ \[3 x float\] \}`).MatchString(ir) {
		t.Error("a FixedArray of f32 is not a [3 x float] aggregate")
	}
	if !strings.Contains(functionBody(t, ir, "main.accumulate"), "fadd float") {
		t.Error("accumulate does not add in single precision")
	}
	if !regexp.MustCompile(`fcmp (oeq|olt|ogt|ole|oge|une) float`).MatchString(ir) {
		t.Error("no comparison on float registers")
	}
}
