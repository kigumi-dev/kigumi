package runtimegen

import (
	"strings"
	"testing"
)

// TestLayoutThreadsTheEmittedHeader checks Layout takes the module's header as
// an argument and keeps only its %/@ lines.
func TestLayoutThreadsTheEmittedHeader(t *testing.T) {
	base, err := Layout("")
	if err != nil {
		t.Fatal(err)
	}
	emitted := "; module header\n%rt_type = type { ptr }\n@.s = private constant [1 x i8] zeroinitializer\n"
	other, err := Layout(emitted)
	if err != nil {
		t.Fatal(err)
	}
	if other == base {
		t.Error("the emitted header does not reach the layout text")
	}
	for _, want := range []string{"%rt_type = type { ptr }", "@.s = private constant"} {
		if !strings.Contains(other, want) {
			t.Errorf("layout text lacks the emitted header line %q", want)
		}
	}
	if strings.Contains(other, "; module header") {
		t.Error("layout text keeps a non-%/@ emitted header line")
	}
}

// TestAbiFingerprintMatchesPlaceholderLength keeps the placeholder the emitted
// module carries usable for the fingerprint's fixed-width swap.
func TestAbiFingerprintMatchesPlaceholderLength(t *testing.T) {
	if want := "kgabi1:" + strings.Repeat("0", 32); AbiPlaceholder != want {
		t.Errorf("placeholder is %q, want %q", AbiPlaceholder, want)
	}
	fp := AbiFingerprint("canonical", "layout", 7, nil)
	if len(fp) != len(AbiPlaceholder) {
		t.Errorf("fingerprint %q and placeholder %q differ in length", fp, AbiPlaceholder)
	}
	if fp == AbiFingerprint("canonical", "layout2", 7, nil) {
		t.Error("a different layout left the fingerprint unchanged")
	}
}
