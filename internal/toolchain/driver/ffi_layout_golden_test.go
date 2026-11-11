package driver_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/driver"
)

// ffiLayoutSrc measures the layout(C) records whose golden size/alignment
// the C compiler pins below.
const ffiLayoutSrc = `import ffi from std/ffi

type S1 layout(C) = {
    pub a u8
    pub b u64
    pub c u8
    pub d f64
    pub e u16
}

type S2 layout(C) = {
    pub a f32
    pub b f64
    pub c u8
}

type S3 layout(C) = {
    pub a u16
    pub b f32
    pub c u8
}

type S4 layout(C) = {
    pub a u64
    pub b u64
    pub c u8
}

type S5 layout(C) = {
    pub a u64
    pub b u64
    pub c u64
}

type Inner layout(C) = {
    pub x u32
    pub y u8
}

type S6 layout(C) = {
    pub a u8
    pub inner Inner
}

type S7 layout(packed) = {
    pub a u8
    pub b u64
}

let s1 = S1 { a: 1, b: 2, c: 3, d: 4.0, e: 5 }
let s2 = S2 { a: 1.0, b: 2.0, c: 3 }
let s3 = S3 { a: 1, b: 2.0, c: 3 }
let s4 = S4 { a: 1, b: 2, c: 3 }
let s5 = S5 { a: 1, b: 2, c: 3 }
let s6 = S6 { a: 1, inner: Inner { x: 2, y: 3 } }
let s7 = S7 { a: 1, b: 2 }

print "${ffi.sizeOf(&s1)}/${ffi.alignOf(&s1)} ${ffi.sizeOf(&s2)}/${ffi.alignOf(&s2)} ${ffi.sizeOf(&s3)}/${ffi.alignOf(&s3)} ${ffi.sizeOf(&s4)}/${ffi.alignOf(&s4)} ${ffi.sizeOf(&s5)}/${ffi.alignOf(&s5)} ${ffi.sizeOf(&s6)}/${ffi.alignOf(&s6)} ${ffi.sizeOf(&s7)}/${ffi.alignOf(&s7)}"
`

// cRecordPrelude declares the nested record; cRecordDecls are the C
// spellings of the measured records in ffiLayoutSrc, in print order.
const cRecordPrelude = "struct Inner { unsigned int x; unsigned char y; };\n"

var cRecordDecls = []struct{ name, decl string }{
	{"S1", "struct S1 { unsigned char a; unsigned long long b; unsigned char c; double d; unsigned short e; };"},
	{"S2", "struct S2 { float a; double b; unsigned char c; };"},
	{"S3", "struct S3 { unsigned short a; float b; unsigned char c; };"},
	{"S4", "struct S4 { unsigned long long a; unsigned long long b; unsigned char c; };"},
	{"S5", "struct S5 { unsigned long long a; unsigned long long b; unsigned long long c; };"},
	{"S6", "struct S6 { unsigned char a; struct Inner in; };"},
	{"S7", "struct S7 { unsigned char a; unsigned long long b; } __attribute__((packed));"},
}

// ffiGolden is the FFI layout corpus: each target's sizeof/alignof for the
// records above, and therefore the values ffi.sizeOf/alignOf must report.
// S1, S2, S4 and S5 are the target-alignment probes: an i386 u64/f64 aligns
// to 4, while x86-64 and wasm32 align them to 8 even with 32-bit pointers.
var ffiGolden = []struct {
	triple string
	sizes  [][2]int
}{
	{"x86_64-linux", [][2]int{{40, 8}, {24, 8}, {12, 4}, {24, 8}, {24, 8}, {12, 4}, {9, 1}}},
	{"aarch64-linux", [][2]int{{40, 8}, {24, 8}, {12, 4}, {24, 8}, {24, 8}, {12, 4}, {9, 1}}},
	{"riscv64-linux", [][2]int{{40, 8}, {24, 8}, {12, 4}, {24, 8}, {24, 8}, {12, 4}, {9, 1}}},
	{"wasm32-wasi", [][2]int{{40, 8}, {24, 8}, {12, 4}, {24, 8}, {24, 8}, {12, 4}, {9, 1}}},
	{"x86-linux-musl", [][2]int{{28, 4}, {16, 4}, {12, 4}, {20, 4}, {24, 4}, {12, 4}, {9, 1}}},
}

// goldenLine formats one target's measurements the way ffiLayoutSrc prints them.
func goldenLine(sizes [][2]int) string {
	parts := make([]string, len(sizes))
	for i, s := range sizes {
		parts[i] = fmt.Sprintf("%d/%d", s[0], s[1])
	}
	return strings.Join(parts, " ") + "\n"
}

// TestFFILayoutGolden runs the FFI layout probe in the interpreter for every
// target in the corpus and compares it with the golden values.
func TestFFILayoutGolden(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"ffi\"\n    kigumi: \"0.1\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte(ffiLayoutSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	std, err := filepath.Abs("../../../std")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range ffiGolden {
		t.Run(c.triple, func(t *testing.T) {
			tg, err := buildconfig.ParseTarget(testCompiler(), c.triple, "")
			if err != nil {
				t.Fatal(err)
			}
			m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: tg})
			if err != nil {
				t.Fatal(err)
			}
			a, err := driver.Analyze(m)
			if err != nil {
				t.Fatal(err)
			}
			if a.Res.HasErrors() {
				t.Fatal(a.Res.Render())
			}
			var out, errOut bytes.Buffer
			if code, err := vm.Run(a.Prog, vm.Options{}, &out, &errOut); err != nil || code != 0 {
				t.Fatalf("run: %v code=%d %s", err, code, errOut.String())
			}
			if want := goldenLine(c.sizes); out.String() != want {
				t.Errorf("got %q, want %q", out.String(), want)
			}
		})
	}
}

// TestFFILayoutGoldenMatchesC proves the golden values with the C compiler:
// the same records declared in C must have exactly those sizes and
// alignments. A triple the compiler cannot target is skipped by name.
func TestFFILayoutGoldenMatchesC(t *testing.T) {
	cc := testCompiler().CCompiler()
	if cc == nil {
		t.Skip("needs a C compiler")
	}
	for _, c := range ffiGolden {
		t.Run(c.triple, func(t *testing.T) {
			parsed, err := buildconfig.ParseTarget(testCompiler(), c.triple, "")
			if err != nil {
				t.Fatal(err)
			}
			b := strings.Builder{}
			b.WriteString(cRecordPrelude)
			for _, d := range cRecordDecls {
				b.WriteString(d.decl)
				b.WriteString("\n")
			}
			if len(c.sizes) != len(cRecordDecls) {
				t.Fatalf("%d golden sizes for %d records", len(c.sizes), len(cRecordDecls))
			}
			for i, d := range cRecordDecls {
				size, align := c.sizes[i][0], c.sizes[i][1]
				fmt.Fprintf(&b, "_Static_assert(sizeof(struct %s) == %d, \"sizeof %s\");\n", d.name, size, d.name)
				fmt.Fprintf(&b, "_Static_assert(_Alignof(struct %s) == %d, \"alignof %s\");\n", d.name, align, d.name)
			}
			checkCScalarSource(t, cc, parsed.CompileTriple(), b.String())
		})
	}
}

// TestFFILayoutGoldenNative lowers the same probe and reads the size/align
// constants out of the IR, so the AOT backend's cLayoutOf agrees with the
// golden values as well as the interpreter does.
func TestFFILayoutGoldenNative(t *testing.T) {
	rtInt := regexp.MustCompile(`call ptr @rt_int\(i64 (-?\d+), i32 \d+\)`)
	for _, c := range ffiGolden {
		t.Run(c.triple, func(t *testing.T) {
			ir := compileTriple(t, c.triple, ffiLayoutSrc)
			var got []string
			for _, m := range rtInt.FindAllStringSubmatch(ir, -1) {
				got = append(got, m[1])
			}
			var want []string
			for _, s := range c.sizes {
				want = append(want, strconv.Itoa(s[0]), strconv.Itoa(s[1]))
			}
			if !containsSequence(got, want) {
				t.Errorf("IR does not report the golden sizes/alignments %v; rt_int sequence was %v", want, got)
			}
		})
	}
}

// containsSequence reports whether needle appears as a contiguous run in hay.
func containsSequence(hay, needle []string) bool {
	for i := 0; i+len(needle) <= len(hay); i++ {
		ok := true
		for j, want := range needle {
			if hay[i+j] != want {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}
