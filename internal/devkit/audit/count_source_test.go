package audit

import "testing"

func TestTestKind(t *testing.T) {
	for _, tc := range []struct {
		name string
		want string
	}{
		{"Test", "test"},
		{"Benchmark", "benchmark"},
		{"Fuzz", "fuzz"},
		{"TestFoo", "test"},
		{"Testfoo", ""},
		{"Tests", ""},
		{"Helper", ""},
	} {
		if got := testKind(tc.name); got != tc.want {
			t.Errorf("testKind(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}
