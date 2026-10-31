package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

func load386(t *testing.T, src string) *driver.Module {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"w\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	std, _ := filepath.Abs("../../../std")
	target, err := driver.ParseTarget(testCompiler(), "x86-linux-musl", "")
	if err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// TestTargetLayout follows usize through a 32 bit target: it stays 64 bits
// wide, and an extern signature uses i32 behind a range check.
func TestTargetLayout(t *testing.T) {
	t.Parallel()
	if driver.HostTarget().Layout().PtrBits != 64 {
		t.Skip("host is not 64 bit")
	}
	res, err := driver.Check(load386(t, "let x: usize = 4294967296\nprint \"${x}\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if res.HasErrors() {
		t.Fatal("usize is 64 bits on every target, so a 33 bit literal fits")
	}
	m := load386(t, "extern(C) {\n    fn kg_len(n: usize) -> usize\n}\n\nlet n: usize = 7\nprint \"${unsafe { kg_len(n) }}\"\n")
	var stderr bytes.Buffer
	ir, ok, err := driver.Compile(m, &stderr)
	if err != nil || !ok {
		t.Fatalf("compile: %v\n%s", err, stderr.String())
	}
	if !strings.Contains(ir, "@kg_len(i32") {
		t.Fatalf("usize should be i32 on a 32 bit target:\n%s", grepLines(ir, "kg_len"))
	}
	if !strings.Contains(ir, "usize value does not fit the size_t of this target") {
		t.Fatal("the usize argument is not range-checked before the i32 call")
	}
}

// TestTargetLayoutCValueCalloc covers runtime-libc-1: ffi.CValue.new must
// emit calloc sized to the target's usize width, not a hardcoded i64,
// since libc's calloc takes size_t = i32 on a 32 bit target.
func TestTargetLayoutCValueCalloc(t *testing.T) {
	if driver.HostTarget().Layout().PtrBits != 64 {
		t.Skip("host is not 64 bit")
	}
	m := load386(t, "import ffi from std/ffi\n\nlet cv = ffi.CValue.new[i64](7)\nprint \"${cv.get()}\"\n")
	var stderr bytes.Buffer
	ir, ok, err := driver.Compile(m, &stderr)
	if err != nil || !ok {
		t.Fatalf("compile: %v\n%s", err, stderr.String())
	}
	if !strings.Contains(ir, "declare ptr @calloc(i32, i32)") {
		t.Fatalf("calloc should use usize width (i32) on a 32 bit target:\n%s", grepLines(ir, "calloc"))
	}
	if strings.Contains(ir, "@calloc(i64") {
		t.Fatalf("calloc must not hardcode i64 on a 32 bit target:\n%s", grepLines(ir, "calloc"))
	}
}

func grepLines(text, needle string) string {
	var out []string
	for _, l := range strings.Split(text, "\n") {
		if strings.Contains(l, needle) {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}
