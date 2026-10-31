package target

import "testing"

func TestScalarAlign(t *testing.T) {
	cases := []struct {
		os, arch string
		i64, f64 int
		i128     int
	}{
		{"linux", "amd64", 8, 8, 16},
		{"linux", "386", 4, 4, 16},
		{"windows", "386", 8, 8, 16},
		{"wasi", "wasm32", 8, 8, 16},
		{"linux", "arm", 8, 8, 8},
		{"linux", "arm64", 8, 8, 16},
		{"linux", "riscv64", 8, 8, 16},
	}
	for _, c := range cases {
		l := ForArch(c.os, c.arch)
		if got := l.ScalarAlign(false, 64); got != c.i64 {
			t.Errorf("%s/%s i64 align = %d, want %d", c.os, c.arch, got, c.i64)
		}
		if got := l.ScalarAlign(true, 64); got != c.f64 {
			t.Errorf("%s/%s f64 align = %d, want %d", c.os, c.arch, got, c.f64)
		}
		if got := l.ScalarAlign(false, 128); got != c.i128 {
			t.Errorf("%s/%s i128 align = %d, want %d", c.os, c.arch, got, c.i128)
		}
		if got := l.ScalarAlign(false, 32); got != 4 {
			t.Errorf("%s/%s i32 align = %d, want 4", c.os, c.arch, got)
		}
	}
}
