package driver_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"kigumi/internal/lang/target"
	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

func compileTriple(t *testing.T, triple, src string) string {
	t.Helper()
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"w\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644)
	std, _ := filepath.Abs("../../../std")
	tg, err := buildconfig.ParseTarget(testCompiler(), triple, "")
	if err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: tg})
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	ir, ok, err := driver.Compile(m, &stderr)
	if err != nil || !ok {
		t.Fatalf("compile for %s: %v\n%s", triple, err, stderr.String())
	}
	return ir
}

const cLayoutSrc = `import ffi from std/ffi

type S layout(C) = {
    pub a u8
    pub b u64
    pub c u8
    pub d f64
    pub e u16
}

let s = S { a: 1, b: 2, c: 3, d: 4.0, e: 5 }
print "${ffi.sizeOf(&s)} ${ffi.alignOf(&s)}"
`

// A layout(C) record's size and alignment come from the target: an i386
// SysV u64 or f64 aligns to 4, x86_64 and wasm32 align them to 8.
func TestCLayoutFollowsTargetAlignment(t *testing.T) {
	cases := []struct {
		triple      string
		size, align string
	}{
		{"x86-linux", "28", "4"},
		{"x86_64-linux", "40", "8"},
		{"wasm32-wasi", "40", "8"},
		{"aarch64-linux", "40", "8"},
	}
	for _, c := range cases {
		t.Run(c.triple, func(t *testing.T) {
			t.Parallel()
			ir := compileTriple(t, c.triple, cLayoutSrc)
			for _, want := range []string{"call ptr @rt_int(i64 " + c.size + ", i32 64)", "call ptr @rt_int(i64 " + c.align + ", i32 64)"} {
				if !strings.Contains(ir, want) {
					t.Errorf("IR for %s lacks %q", c.triple, want)
				}
			}
		})
	}
}

// The embedded datalayout table agrees with the one the C compiler reports.
func TestTargetDataLayoutMatchesCompiler(t *testing.T) {
	cc := testCompiler().CCompiler()
	if cc == nil {
		t.Skip("needs a C compiler")
	}
	triples := []string{"x86_64-linux", "x86-linux", "x86-windows", "aarch64-linux", "riscv64-linux", "wasm32-wasi", "arm-linux-gnueabihf"}
	dir := t.TempDir()
	src := filepath.Join(dir, "e.c")
	os.WriteFile(src, []byte("int x;\n"), 0o644)
	re := regexp.MustCompile(`target datalayout = "([^"]*)"`)
	for _, triple := range triples {
		out := filepath.Join(dir, "e.ll")
		args := append(cc[1:len(cc):len(cc)], "-target", triple, "-S", "-emit-llvm", "-o", out, src)
		if b, err := exec.Command(cc[0], args...).CombinedOutput(); err != nil {
			t.Skipf("%s cannot target %s: %v\n%s", cc[0], triple, err, b)
		}
		ll, _ := os.ReadFile(out)
		m := re.FindSubmatch(ll)
		if m == nil {
			t.Fatalf("no datalayout in the IR for %s", triple)
		}
		parsed, err := buildconfig.ParseTarget(testCompiler(), triple, "")
		if err != nil {
			t.Fatal(err)
		}
		l := parsed.Layout()
		for _, bits := range []int{8, 16, 32, 64, 128} {
			if got, want := l.ScalarAlign(false, bits), target.ScalarAlignIn(string(m[1]), false, bits); got != want {
				t.Errorf("%s: i%d align = %d, compiler says %d", triple, bits, got, want)
			}
		}
		for _, bits := range []int{32, 64} {
			if got, want := l.ScalarAlign(true, bits), target.ScalarAlignIn(string(m[1]), true, bits); got != want {
				t.Errorf("%s: f%d align = %d, compiler says %d", triple, bits, got, want)
			}
		}
		if got := l.PtrBits; got != 64 && got != 32 {
			t.Errorf("%s: pointer width %d", triple, got)
		}
	}
}

const usizeCallSrc = `extern(C) {
    fn malloc(n: usize) -> *mut u8
    fn labs(n: isize) -> isize
}

let big: u64 = 4294967312
let p = unsafe { malloc(big.toUsize()) }
let m: isize = -5
print "${unsafe { labs(m) }}"
`

// usize and isize stay 64 bits on every target; passing one to a 32-bit
// size_t or ptrdiff_t parameter is range-checked instead of truncated.
func TestUsizeToCRangeChecked(t *testing.T) {
	for _, triple := range []string{"wasm32-wasi", "x86-linux"} {
		t.Run(triple, func(t *testing.T) {
			t.Parallel()
			ir := compileTriple(t, triple, usizeCallSrc)
			for _, want := range []string{"declare ptr @malloc(i32)", "declare i32 @labs(i32)", "usize value does not fit the size_t of this target", "isize value does not fit the ptrdiff_t of this target"} {
				if !strings.Contains(ir, want) {
					t.Errorf("IR for %s lacks %q", triple, want)
				}
			}
		})
	}
	t.Run("x86_64-linux", func(t *testing.T) {
		t.Parallel()
		ir := compileTriple(t, "x86_64-linux", usizeCallSrc)
		if !strings.Contains(ir, "declare ptr @malloc(i64)") || strings.Contains(ir, "does not fit the size_t") {
			t.Error("a 64-bit target passes usize unchecked as i64")
		}
	})
}
