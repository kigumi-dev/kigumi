package driver

import (
	"strings"
	"testing"
)

func TestNativeOnlyRefusal(t *testing.T) {
	cases := []struct {
		why        string
		nativeOnly bool
	}{
		{"callc", true},
		{"asm", true},
		{"cfnptr", true},
		{"primitive os.Host.dl", true},
		{"primitive dl.Symbol.call", true},
		{"primitive ffi.CString.new", true},
		{"foreign function dlopen", true},
		{"primitive os.Host.net", false},
		{"primitive net.Net.http", false},
		{"builtin comptime", false},
		{"an unevaluated comptime block", false},
	}
	for _, c := range cases {
		t.Run(c.why, func(t *testing.T) {
			if got := nativeOnlyRefusal(c.why); got != c.nativeOnly {
				t.Errorf("nativeOnlyRefusal(%q) = %v, want %v", c.why, got, c.nativeOnly)
			}
			err := vmRefusalError(c.why)
			wantSubstr := "run with --engine interp"
			if c.nativeOnly {
				wantSubstr = "kigumi build"
			}
			if !strings.Contains(err.Error(), wantSubstr) {
				t.Errorf("vmRefusalError(%q) = %q, want it to mention %q", c.why, err.Error(), wantSubstr)
			}
		})
	}
}
