package driver_test

import (
	"strings"
	"testing"

	"kigumi/internal/driver"
)

func TestSummaryRender(t *testing.T) {
	s := driver.Summary{
		Policy:       driver.Development(),
		Dependencies: []string{"greet v1.0.0"},
	}
	out := s.Render()
	for _, want := range []string{
		"policy: development\n",
		"dependencies: greet v1.0.0\n",
		"local overrides: none\n",
		"host inputs: none\n",
		"build isolation: trusted host (not sandboxed)\n",
		"reproducibility: not verified (pass --reproducible to verify)\n",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "hermetic") || strings.Contains(out, "sandbox)") && !strings.Contains(out, "not sandboxed") {
		t.Fatalf("summary must never claim hermetic/sandboxed isolation:\n%s", out)
	}

	rel := driver.Summary{Policy: driver.Release(), ReproducibleChecked: true, Reproducible: true}
	relOut := rel.Render()
	if !strings.Contains(relOut, "policy: release-v1\n") {
		t.Fatalf("missing release policy line:\n%s", relOut)
	}
	if !strings.Contains(relOut, "reproducibility: verified (two independent builds produced identical output)\n") {
		t.Fatalf("missing verified reproducibility line:\n%s", relOut)
	}
}
