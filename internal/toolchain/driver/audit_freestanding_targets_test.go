package driver_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

func largeAggregateProgram(fields int) string {
	var decl, init []string
	for i := range fields {
		decl = append(decl, fmt.Sprintf("    pub f%d Int", i))
		init = append(init, fmt.Sprintf("f%d: %d", i, i))
	}
	return "type Big = {\n" + strings.Join(decl, "\n") + "\n}\n\n" +
		"fn id[T](x: T) -> T {\n    x\n}\n\n" +
		"fn pass[T](x: T) -> i32 {\n    let y = id(x)\n    1\n}\n\n" +
		"export(C) {\n    pub fn run() -> i32 {\n        let b = Big { " + strings.Join(init, ", ") + " }\n        pass(b)\n    }\n}\n"
}

const (
	minimalProgram = "export(C) {\n    pub fn answer() -> i32 {\n        7\n    }\n}\n"

	fixedArrayProgram = "fn id[T](x: T) -> T {\n    x\n}\n\n" +
		"fn pass[T](x: T) -> i32 {\n    let y = id(x)\n    1\n}\n\n" +
		"export(C) {\n    pub fn run() -> i32 {\n        let a = FixedArray.filled[Int, 64](3)\n        pass(a)\n    }\n}\n"

	arithmeticProgram = "export(C) {\n    pub fn mix(a: Int, b: Int, f: Float) -> Float {\n" +
		"        (a / b + a % b).toFloat() * f\n    }\n\n" +
		"    pub fn wide(a: u64, b: u64) -> i32 {\n        let x: u128 = a.toU128()\n        let y: u128 = b.toU128()\n        let z = x * y + x / (y + 1)\n        if z > x { 1 } else { 0 }\n    }\n}\n"
)

var freestandingTargetPrograms = []struct{ name, src string }{
	{"minimal", minimalProgram},
	{"large record through generic", largeAggregateProgram(40)},
	{"fixed array through generic", fixedArrayProgram},
	{"wide arithmetic", arithmeticProgram},
}

// Every freestanding target the table accepts, each as its own subtest so a regression names the target.
var freestandingTargets = []string{
	"x86_64-freestanding", "x86-freestanding", "arm-freestanding", "aarch64-freestanding", "riscv64-freestanding", "wasm32-freestanding",
	"x86_64-none", "x86-none", "arm-none", "aarch64-none", "riscv64-none", "wasm32-none",
	"x86_64-uefi", "x86-uefi", "arm-uefi", "aarch64-uefi", "riscv64-uefi",
}

// The backend lowers a large aggregate copy to memmove, and a 32-bit or wasm target calls compiler-rt helpers; the audit must accept both.
func TestBuildFreestandingTargetsPassAudit(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, triple := range freestandingTargets {
		for _, p := range freestandingTargetPrograms {
			t.Run(triple+"/"+p.name, func(t *testing.T) {
				t.Parallel()
				root := t.TempDir()
				os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"audit\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
				os.WriteFile(filepath.Join(root, "main.kg"), []byte(p.src), 0o644)
				target, err := driver.ParseTarget(testCompiler(), triple, "")
				if err != nil {
					t.Fatal(err)
				}
				std, _ := filepath.Abs("../../../std")
				m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
				if err != nil {
					t.Fatal(err)
				}
				var buildErr bytes.Buffer
				if ok, err := driver.BuildWith(m, filepath.Join(root, "audit.a"), &buildErr, driver.BuildOptions{Target: target}); err != nil || !ok {
					t.Fatalf("build: %v\n%s", err, buildErr.String())
				}
			})
		}
	}
}

func TestParseTargetRejectsWasmUEFI(t *testing.T) {
	if _, err := driver.ParseTarget(testCompiler(), "wasm32-uefi", ""); err == nil || !strings.Contains(err.Error(), "wasm32 has no UEFI") {
		t.Fatalf("want a wasm32 UEFI rejection, got %v", err)
	}
}

// The audit reads wasm and 32-bit COFF objects too, so an undeclared libc call is named on them as it is on ELF.
func TestAuditRejectsDisallowedSymbolOnObjectFormats(t *testing.T) {
	if testCompiler().CCompiler() == nil {
		t.Skip("no C compiler")
	}
	for _, triple := range []string{"wasm32-freestanding", "x86-uefi", "arm-freestanding"} {
		t.Run(triple, func(t *testing.T) {
			t.Parallel()
			root := writeAuditModule(t, "leaky", "extern int printf(const char *fmt, ...);\nint leak(void) { return printf(\"hi\\n\"); }\n")
			target, err := driver.ParseTarget(testCompiler(), triple, "")
			if err != nil {
				t.Fatal(err)
			}
			std, _ := filepath.Abs("../../../std")
			m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
			if err != nil {
				t.Fatal(err)
			}
			var buildErr bytes.Buffer
			ok, err := driver.BuildWith(m, filepath.Join(root, "leaky.a"), &buildErr, driver.BuildOptions{Target: target})
			if ok || err == nil || !strings.Contains(err.Error(), `"printf"`) {
				t.Fatalf("want an audit failure naming printf, got ok=%v err=%v", ok, err)
			}
		})
	}
}
