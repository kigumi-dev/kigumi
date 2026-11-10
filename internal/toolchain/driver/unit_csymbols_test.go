package driver_test

import (
	"strings"
	"testing"
)

// TestUnitCheckSeesCrossPackageCSymbolConflict: E2218 is a whole-module rule, so a unit that re-checks one package must not lose a
// duplicate C name it can only see in a package it replays from a clean record.
func TestUnitCheckSeesCrossPackageCSymbolConflict(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	files := map[string]string{
		"mod.kg":          "Module {\n    name: \"csym\"\n    kigumi: \"0.1\"\n}\n",
		"alpha/alpha.kg":  "export(C) {\n    @linkName(\"probe_shared\")\n    pub fn a() -> Int {\n        1\n    }\n}\n",
		"beta/beta.kg":    "export(C) {\n    @linkName(\"probe_beta\")\n    pub fn b() -> Int {\n        2\n    }\n}\n",
		"cmd/app/main.kg": "import alpha from csym/alpha\nimport beta from csym/beta\n\nprint \"${alpha.a() + beta.b()}\"\n",
	}
	writeGraph(t, root, files)
	if _, got, failed := unitCheck(t, root, cache, "cmd/app/main.kg"); got != "" || failed {
		t.Fatalf("distinct C names must check clean, got %q (errors=%v)", got, failed)
	}
	// Only beta's source changes: its public API stays the same, so every dependant replays from its record.
	writeGraph(t, root, map[string]string{"beta/beta.kg": "export(C) {\n    @linkName(\"probe_shared\")\n    pub fn b() -> Int {\n        2\n    }\n}\n"})
	_, got, failed := unitCheck(t, root, cache, "cmd/app/main.kg")
	if !failed || !strings.Contains(got, "E2218") {
		t.Fatalf("a duplicate C symbol across two packages must fail the unit check, got %q (errors=%v)", got, failed)
	}
	want, wantFailed := wholeCheck(t, root, "cmd/app/main.kg")
	if got != want || failed != wantFailed {
		t.Fatalf("unit check differs from the whole check\n--- unit (errors=%v)\n%s--- whole (errors=%v)\n%s", failed, got, wantFailed, want)
	}
}
