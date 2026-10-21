package cheader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEvalConst(t *testing.T) {
	abi := ABIFor("linux", "amd64")
	cases := []struct{ in, typ, lit string }{
		{"-@as(c_int, 5) - @as(c_int, 1)", "i32", "-6"},
		{"@as(c_uint, 1) << @as(c_int, 31)", "u32", "2147483648"},
		{"-@as(c_int, 32767) - @as(c_int, 1)", "i32", "-32768"},
		{"-__helpers.promoteIntLiteral(c_int, 2147483647, .decimal) - @as(c_int, 1)", "i32", "-2147483648"},
		{"-__helpers.promoteIntLiteral(c_long, 9223372036854775807, .decimal) - @as(c_int, 1)", "i64", "-9223372036854775808"},
		{"__helpers.promoteIntLiteral(c_int, 100000000000, .decimal)", "i64", "100000000000"},
		{"__helpers.promoteIntLiteral(c_int, 0xff00, .hex) | @as(c_int, 0x00ff)", "i32", "65535"},
		{"@as(c_int, 0x7fffffff)", "i32", "0x7fffffff"},
		{"-@as(c_int, 128)", "i32", "-128"},
		{"-@as(c_uint, 1)", "u32", "4294967295"},
		{"__helpers.promoteIntLiteral(c_int, 0xffffffff, .hex) + @as(c_int, 1)", "u32", "0"},
		{"~__helpers.promoteIntLiteral(c_int, 0xffffffff, .hex)", "u32", "0"},
		{"-__helpers.promoteIntLiteral(c_int, 0x80000000, .hex)", "u32", "2147483648"},
		{"__helpers.promoteIntLiteral(c_int, 4294967296, .decimal)", "i64", "4294967296"},
		{`"\x1b[0m"`, "String", `"\u{1b}[0m"`},
		{`"nul\x00end"`, "String", `"nul\0end"`},
		{`"cost: ${x}"`, "String", `"cost: \${x}"`},
		{"@as(f32, 1.5)", "f32", "1.5"},
		{`"a" ++ "b"`, "String", `"ab"`},
		{"__INT64_C(__helpers.promoteIntLiteral(c_int, 9223372036854775807, .decimal))", "i64", "9223372036854775807"},
		{"-__INT64_C(__helpers.promoteIntLiteral(c_int, 9223372036854775807, .decimal)) - @as(c_int, 1)", "i64", "-9223372036854775808"},
		{"__UINT64_C(__helpers.promoteIntLiteral(c_int, 18446744073709551615, .decimal))", "u64", "18446744073709551615"},
		{"UINT32_C(@as(c_int, 5))", "u32", "5"},
		{"UINT64_C(@as(c_int, 5))", "u64", "5"},
		{"INT64_C(@as(c_int, 5))", "i64", "5"},
		{"INT32_C(@as(c_int, 7))", "i32", "7"},
		{"UINT16_C(@as(c_int, 7))", "i32", "7"},
		{"UINT32_C(__helpers.promoteIntLiteral(c_int, 4294967295, .decimal))", "u32", "4294967295"},
		{"INT64_C(__helpers.promoteIntLiteral(c_int, 0xffffffffffffffff, .hex))", "u64", "18446744073709551615"},
		{"UINT64_C(@as(c_int, 1)) << @as(c_int, 63)", "u64", "9223372036854775808"},
	}
	for _, c := range cases {
		typ, lit, ok := evalConst(c.in, abi)
		if !ok || typ != c.typ || lit != c.lit {
			t.Errorf("%s: got %s %s %v, want %s %s", c.in, typ, lit, ok, c.typ, c.lit)
		}
	}
	for _, in := range []string{"@as(c_int, 2147483647) + @as(c_int, 1)", "@as(c_int, 1) / @as(c_int, 0)", "@as(c_int, 1) << @as(c_int, 32)", "foo", "@as(c_int, 1) +", `"\xff"`, `"\q"`, "UINT32_C(-@as(c_int, 1))", "UINT64_C(@as(c_int, 1)", "INT64_C(__helpers.promoteIntLiteral(c_int, 18446744073709551616, .decimal))", "UINT64_C"} {
		if _, _, ok := evalConst(in, abi); ok {
			t.Errorf("%s: want rejection", in)
		}
	}
}

func TestEvalConstResolvesEarlierConstants(t *testing.T) {
	abi := ABIFor("linux", "amd64")
	vals := map[string]cval{"INT64_MAX": {v: maxInt64, bits: 64, signed: true}}
	typ, lit, _, ok := evalConstEnv("INT64_MAX", abi, vals)
	if !ok || typ != "i64" || lit != "9223372036854775807" {
		t.Errorf("got %s %s %v", typ, lit, ok)
	}
	if _, _, _, ok := evalConstEnv("INT64_MAX + @as(c_int, 1)", abi, vals); ok {
		t.Error("an overflowing sum of constants must be rejected")
	}
	if _, _, _, ok := evalConstEnv("UNKNOWN", abi, vals); ok {
		t.Error("an unknown name must be rejected")
	}
}

func TestGenerateSkipsWideStringMacros(t *testing.T) {
	full := `pub const INT64_MAX = __INT64_C(__helpers.promoteIntLiteral(c_int, 9223372036854775807, .decimal));
pub const BIG = INT64_MAX;
pub const W = "x";
pub const N = "plain";
`
	src, skipped := GenerateWith(full, "", ABIFor("linux", "amd64"), map[string]bool{"W": true})
	for _, want := range []string{"const INT64_MAX: i64 = 9223372036854775807", "const BIG: i64 = 9223372036854775807", `const N: String = "plain"`} {
		if !strings.Contains(src, want) {
			t.Errorf("missing %q in:\n%s", want, src)
		}
	}
	if strings.Contains(src, "const W") {
		t.Errorf("wide macro imported:\n%s", src)
	}
	if len(skipped) != 1 || !strings.HasPrefix(skipped[0], "W: wide string literal") {
		t.Errorf("skipped = %v", skipped)
	}
}

func TestWideStringMacrosScansDefines(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "w.h")
	os.WriteFile(path, []byte("#define A L\"a\"\n# define B (u\"b\")\n#define C U\"c\"\n#define D u8\"d\"\n#define E \\\n  L\"e\"\n#define F \"f\"\n#define G \"a\" L\"b\"\n#define H /* c */ L\"c\"\n#define I \" L\"\n#define J \"x\" // L\"y\"\n#define K (U\"k\")\n"), 0o644)
	got := wideStringMacros([]SourceFile{{Path: path}})
	for _, n := range []string{"A", "B", "C", "E", "G", "H", "K"} {
		if !got[n] {
			t.Errorf("%s not detected: %v", n, got)
		}
	}
	for _, n := range []string{"D", "F", "I", "J"} {
		if got[n] {
			t.Errorf("%s is not wide: %v", n, got)
		}
	}
}
