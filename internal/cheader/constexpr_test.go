package cheader

import "testing"

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
	}
	for _, c := range cases {
		typ, lit, ok := evalConst(c.in, abi)
		if !ok || typ != c.typ || lit != c.lit {
			t.Errorf("%s: got %s %s %v, want %s %s", c.in, typ, lit, ok, c.typ, c.lit)
		}
	}
	for _, in := range []string{"@as(c_int, 2147483647) + @as(c_int, 1)", "@as(c_int, 1) / @as(c_int, 0)", "@as(c_int, 1) << @as(c_int, 32)", "foo", "@as(c_int, 1) +", `"\xff"`, `"\q"`} {
		if _, _, ok := evalConst(in, abi); ok {
			t.Errorf("%s: want rejection", in)
		}
	}
}
