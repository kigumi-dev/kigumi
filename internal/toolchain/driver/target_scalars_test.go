package driver_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/lang/target"
)

// cScalarChecks names the C spelling of each scalar the layout models.
var cScalarChecks = []struct {
	name string
	decl string
	s    target.Scalar
}{
	{"char", "char", target.CChar},
	{"short", "short", target.CShort},
	{"int", "int", target.CInt},
	{"long", "long", target.CLong},
	{"long long", "long long", target.CLongLong},
	{"float", "float", target.CFloat},
	{"double", "double", target.CDouble},
	{"long double", "long double", target.CLongDouble},
	{"pointer", "void *", target.CPointer},
}

// cScalarSource writes _Static_asserts for every C size and alignment the
// layout claims, plus the byte order, so the C compiler is the oracle.
func cScalarSource(l target.Layout) string {
	var b strings.Builder
	for _, c := range cScalarChecks {
		fmt.Fprintf(&b, "_Static_assert(sizeof(%s) == %d, \"sizeof %s\");\n", c.decl, l.CSize(c.s), c.name)
		fmt.Fprintf(&b, "_Static_assert(_Alignof(%s) == %d, \"alignof %s\");\n", c.decl, l.CAlign(c.s), c.name)
	}
	if l.C.Int128 {
		fmt.Fprintf(&b, "_Static_assert(sizeof(__int128) == %d, \"sizeof __int128\");\n", l.CSize(target.CInt128))
		fmt.Fprintf(&b, "_Static_assert(_Alignof(__int128) == %d, \"alignof __int128\");\n", l.CAlign(target.CInt128))
	}
	if l.BigEndian {
		b.WriteString("#if __BYTE_ORDER__ != __ORDER_BIG_ENDIAN__\n#error byte order\n#endif\n")
	} else {
		b.WriteString("#if __BYTE_ORDER__ != __ORDER_LITTLE_ENDIAN__\n#error byte order\n#endif\n")
	}
	return b.String()
}

// TestTargetScalarModelMatchesCompiler checks the C scalar model and byte
// order of every target the layout table knows against clang: a fact the
// compiler disagrees with fails to compile. A target this compiler cannot
// cross-compile to is skipped by name.
func TestTargetScalarModelMatchesCompiler(t *testing.T) {
	cc := testCompiler().CCompiler()
	if cc == nil {
		t.Skip("needs a C compiler")
	}
	cases := []struct {
		name   string
		triple string
		layout target.Layout
	}{
		{"x86_64-linux", "x86_64-linux", target.ForArch("linux", "amd64")},
		{"x86-linux", "x86-linux", target.ForArch("linux", "386")},
		{"aarch64-linux", "aarch64-linux", target.ForArch("linux", "arm64")},
		{"arm-linux", "arm-linux-gnueabihf", target.ForArch("linux", "arm")},
		{"riscv64-linux", "riscv64-linux", target.ForArch("linux", "riscv64")},
		{"wasm32-wasi", "wasm32-wasi", target.ForArch("wasi", "wasm32")},
		{"x86_64-windows-gnu", "x86_64-windows-gnu", target.ForOSABI("windows", "amd64", "gnu")},
		{"x86_64-windows-msvc", "x86_64-windows-msvc", target.ForOSABI("windows", "amd64", "msvc")},
		{"x86-windows-gnu", "x86-windows-gnu", target.ForOSABI("windows", "386", "gnu")},
		{"x86-windows-msvc", "x86-windows-msvc", target.ForOSABI("windows", "386", "msvc")},
		{"aarch64-windows-msvc", "aarch64-windows-msvc", target.ForOSABI("windows", "arm64", "msvc")},
		{"x86_64-uefi", "x86_64-uefi", target.ForArch("uefi", "amd64")},
		{"x86-uefi", "x86-uefi", target.ForArch("uefi", "386")},
		{"aarch64-uefi", "aarch64-uefi", target.ForArch("uefi", "arm64")},
		{"arm-uefi", "arm-uefi", target.ForArch("uefi", "arm")},
		{"s390x-linux", "s390x-linux", target.ForArch("linux", "s390x")},
		{"powerpc64-linux", "powerpc64-linux", target.ForArch("linux", "ppc64")},
		{"powerpc64le-linux", "powerpc64le-linux", target.ForArch("linux", "ppc64le")},
		{"mips-linux", "mips-linux", target.ForArch("linux", "mips")},
		{"mipsel-linux", "mipsel-linux", target.ForArch("linux", "mipsle")},
		{"mips64-linux", "mips64-linux", target.ForArch("linux", "mips64")},
		{"mips64el-linux", "mips64el-linux", target.ForArch("linux", "mips64le")},
		{"loongarch64-linux", "loongarch64-linux", target.ForArch("linux", "loong64")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			checkCScalarSource(t, cc, c.triple, cScalarSource(c.layout))
		})
	}
}

// checkCScalarSource compiles a static-assert file for triple; a triple the
// compiler cannot target is skipped.
func checkCScalarSource(t *testing.T, cc []string, triple, asserts string) {
	t.Helper()
	dir := t.TempDir()
	full := filepath.Join(dir, "a.c")
	if err := os.WriteFile(full, []byte(asserts), 0o644); err != nil {
		t.Fatal(err)
	}
	args := append(cc[1:len(cc):len(cc)], "-target", triple, "-c", "-o", filepath.Join(dir, "a.o"), full)
	if b, err := exec.Command(cc[0], args...).CombinedOutput(); err != nil {
		if strings.Contains(string(b), "unable to find") || strings.Contains(string(b), "unknown target") {
			t.Skipf("%s cannot target %s: %v\n%s", cc[0], triple, err, b)
		}
		t.Fatalf("%s disagrees with the layout for %s: %v\n%s\n--- asserts ---\n%s", cc[0], triple, err, b, asserts)
	}
}
