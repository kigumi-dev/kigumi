package driver_test

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// Density thresholds from the examples reorganization plan: every example is
// substantial on its own, the set is not padded, and the median carries the
// weight rather than one large program.
const (
	minExampleLines = 140
	maxExampleLines = 360 // review signal, not a failure
	minTotalLines   = 1600
	maxTotalLines   = 1950
	minMedianLines  = 180
)

// countKigumiLines counts the physical lines of every .kg file under dir.
func countKigumiLines(t *testing.T, dir string) int {
	t.Helper()
	total := 0
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(path) != ".kg" {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		total += strings.Count(string(b), "\n")
		if len(b) > 0 && b[len(b)-1] != '\n' {
			total++
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return total
}

// TestExampleDensity fails when an example is a fragment, when the set is
// padded past the planned total, or when the median is too low to make the
// examples worth reading.
func TestExampleDensity(t *testing.T) {
	root, _ := filepath.Abs("../../../examples")
	_, names, err := driver.LoadExampleCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	sizes := make([]int, 0, len(names))
	total := 0
	for _, name := range names {
		n := countKigumiLines(t, filepath.Join(root, name))
		sizes = append(sizes, n)
		total += n
		if n < minExampleLines {
			t.Errorf("%s: %d Kigumi lines, want at least %d", name, n, minExampleLines)
		}
		if n > maxExampleLines {
			t.Logf("review %s: %d Kigumi lines exceeds the %d-line review signal", name, n, maxExampleLines)
		}
	}
	sort.Ints(sizes)
	median := sizes[len(sizes)/2]
	if len(sizes)%2 == 0 {
		median = (sizes[len(sizes)/2-1] + sizes[len(sizes)/2]) / 2
	}
	if total < minTotalLines || total > maxTotalLines {
		t.Errorf("examples total %d Kigumi lines, want %d..%d", total, minTotalLines, maxTotalLines)
	}
	if median < minMedianLines {
		t.Errorf("example median %d Kigumi lines, want at least %d", median, minMedianLines)
	}
	t.Logf("examples: %d dirs, %d Kigumi lines, median %d", len(sizes), total, median)
}

// TestExampleCoverage checks examples/features.json against the catalog: every
// required tag is demonstrated by at least one example, a blocked tag is never
// claimed (no faking), and unknown tags are failures.
func TestExampleCoverage(t *testing.T) {
	root, _ := filepath.Abs("../../../examples")
	catalog, names, err := driver.LoadExampleCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	required, blocked, err := driver.LoadExampleFeatures(root)
	if err != nil {
		t.Fatal(err)
	}
	covered := make(map[string][]string, len(required))
	for _, name := range names {
		for _, tag := range catalog[name].Features {
			if _, ok := required[tag]; !ok {
				if b, isBlocked := blocked[tag]; isBlocked {
					t.Errorf("%s claims %q, which features.json blocks (%s); a blocked feature must not be faked", name, tag, b.BlockedBy)
				} else {
					t.Errorf("%s claims unknown feature tag %q (not in examples/features.json)", name, tag)
				}
				continue
			}
			covered[tag] = append(covered[tag], name)
		}
	}
	for tag := range required {
		if len(covered[tag]) == 0 {
			t.Errorf("required feature %q is demonstrated by no example", tag)
		}
	}
	for tag, b := range blocked {
		if b.BlockedBy == "" {
			t.Errorf("blocked feature %q has no blocked_by ID", tag)
		}
		if len(covered[tag]) != 0 {
			t.Errorf("blocked feature %q is claimed by %v", tag, covered[tag])
		}
	}
}

// TestBlockedExampleFeatures proves the compiler-level blocker an example
// would otherwise have to fake. `pub(in ...)` parses but is rejected with E133,
// so no example may claim module.visibility.pub-in.
func TestBlockedExampleFeatures(t *testing.T) {
	root, _ := filepath.Abs("../../../examples")
	_, blocked, err := driver.LoadExampleFeatures(root)
	if err != nil {
		t.Fatal(err)
	}
	for tag, b := range blocked {
		if strings.TrimSpace(b.Note) == "" {
			t.Errorf("blocked feature %q has no note explaining the blocker", tag)
		}
	}
	if _, ok := blocked["module.visibility.pub-in"]; !ok {
		t.Fatal("features.json no longer blocks module.visibility.pub-in; its proof is stale")
	}
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "lib"), 0o755)
	os.WriteFile(filepath.Join(dir, "mod.kg"), []byte("Module {\n    name: \"blocked\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "lib", "lib.kg"), []byte("pub(in blocked) fn hidden() -> Int { 1 }\n"), 0o644)
	m, err := driver.LoadModule(dir, driver.LoadOptions{Compiler: testCompiler()})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(res.Render(), "E133") {
		t.Fatalf("pub(in ...) should still be rejected with E133; got:\n%s", res.Render())
	}
}
