package llgen_test

import (
	"regexp"
	"strings"
	"testing"
)

func TestStdAdapterReadsBoundValuesInPlace(t *testing.T) {
	ir := specializedIR(t, `let n: Int = 5
let f = n.toFloat
print "${f()}"
`)
	header := regexp.MustCompile(`define ptr @"[^"]*\$stdbound1\$[^"]*"`).FindString(ir)
	if header == "" {
		t.Fatalf("no adapter with one bound value in:\n%s", ir)
	}
	adapter := bodyOf(t, ir, header)
	if !strings.Contains(adapter, "@rt_iter_at(ptr %env, i64 0, i32 1)") {
		t.Errorf("the adapter does not borrow the bound value from the environment:\n%s", adapter)
	}
	if strings.Contains(adapter, "@rt_at(ptr %env") || strings.Contains(adapter, "@rt_release") {
		t.Errorf("the adapter takes a reference to the bound value and must give it back:\n%s", adapter)
	}
}
