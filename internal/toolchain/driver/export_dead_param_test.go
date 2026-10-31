package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// An export(C) wrapper must reach its body with no allocator call, so a signal handler stays
// async-signal-safe: a dead record parameter lowers to rt_unit(), a scalar one crosses in a register.
func TestExportDeadParamNeedsNoBox(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"w\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	src := `pub type Point layout(C) = {
    pub x i32
    pub y i32
}

export(C) {
    pub fn onSig(sig: i32, p: Point) {}
    pub fn twice(x: i32) -> i32 { x * 2 }
}

print "ok"
`
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: driver.HostTarget()})
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	ir, ok, err := driver.Compile(m, &stderr)
	if err != nil || !ok {
		t.Fatalf("compile: %v\n%s", err, stderr.String())
	}
	dead := defineBlock(ir, "onSig")
	if dead == "" {
		t.Fatalf("no define for onSig:\n%s", grepLines(ir, "onSig"))
	}
	for _, alloc := range []string{"@rt_int(", "@kg_cread_", "@rt_record("} {
		if strings.Contains(dead, alloc) {
			t.Fatalf("dead parameter still boxed through %s:\n%s", alloc, dead)
		}
	}
	wrapper := defineBlock(ir, "kg_export_onSig")
	if !strings.Contains(dead, `define ptr @"w.onSig$kg"(i64 %p0, ptr %p1)`) || !strings.Contains(wrapper, "sext i32 %a0 to i64") {
		t.Fatalf("the dead scalar parameter should cross as its register, not a box:\n%s", dead)
	}
	if !strings.Contains(wrapper, "@rt_unit()") {
		t.Fatalf("the dead record parameter should lower to rt_unit():\n%s", wrapper)
	}
	live := defineBlock(ir, "twice")
	if strings.Contains(live, "@rt_int(") || !strings.Contains(live, `define i64 @"w.twice$kg"(i64 %p0)`) {
		t.Fatalf("a read scalar parameter and the result must cross in registers:\n%s", live)
	}
}

// defineBlock returns the IR of every function whose name contains sym.
func defineBlock(ir, sym string) string {
	var out []string
	in := false
	for _, l := range strings.Split(ir, "\n") {
		if strings.HasPrefix(l, "define ") && strings.Contains(l, sym) {
			in = true
		}
		if in {
			out = append(out, l)
			if l == "}" {
				in = false
			}
		}
	}
	return strings.Join(out, "\n")
}
