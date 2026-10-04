package driver_test

import (
	"strings"
	"testing"

	"kigumi/internal/driver"
)

// TestCheckEngineAuto checks "auto" keeps its own rejection message instead of falling into the generic "unknown engine" branch.
func TestCheckEngineAuto(t *testing.T) {
	err := driver.CheckEngine("auto")
	if err == nil {
		t.Fatal("want an error for --engine auto")
	}
	if !strings.Contains(err.Error(), "\"auto\" was removed") {
		t.Errorf("want the auto-removed message, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "--engine interp") || !strings.Contains(err.Error(), "kigumi build") {
		t.Errorf("want guidance toward --engine interp and kigumi build, got %q", err.Error())
	}
	if got := driver.CheckEngine("auto").Error(); got != err.Error() {
		t.Errorf("CheckEngine(%q) is not stable across calls: %q vs %q", "auto", got, err.Error())
	}

	bogus := driver.CheckEngine("bogus")
	if bogus == nil {
		t.Fatal("want an error for --engine bogus")
	}
	if strings.Contains(bogus.Error(), "\"auto\" was removed") {
		t.Errorf("the generic unknown-engine branch must not reuse the auto-specific message, got %q", bogus.Error())
	}

	for _, ok := range []string{"", "vm", "interp"} {
		if err := driver.CheckEngine(ok); err != nil {
			t.Errorf("CheckEngine(%q) = %v, want nil", ok, err)
		}
	}
}
