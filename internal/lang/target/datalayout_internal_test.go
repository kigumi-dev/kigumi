package target

import (
	"strconv"
	"strings"
	"testing"
)

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

// TestDataLayoutMatchesFacts keeps the datalayout table and the layout facts
// consistent: every architecture has a datalayout whose byte order and
// pointer specification agree with BigEndian and PtrBits.
func TestDataLayoutMatchesFacts(t *testing.T) {
	for arch := range archFactsTable {
		l := ForArch("linux", arch)
		dl := l.DataLayout()
		big := strings.HasPrefix(dl, "E-")
		if big != l.BigEndian {
			t.Errorf("%s: datalayout %q big-endian = %v, want %v", arch, dl, big, l.BigEndian)
		}
		if p, ok := pointerSpec(dl); ok && p != l.PtrBits {
			t.Errorf("%s: datalayout pointer spec %d bits, want %d", arch, p, l.PtrBits)
		}
	}
}

// pointerSpec reads the address width out of the first p: component.
func pointerSpec(dataLayout string) (int, bool) {
	for _, part := range strings.Split(dataLayout, "-") {
		rest, ok := strings.CutPrefix(part, "p:")
		if !ok {
			continue
		}
		fields := strings.Split(rest, ":")
		n, err := strconv.Atoi(fields[0])
		if err != nil {
			return 0, false
		}
		return n, true
	}
	return 0, false
}
