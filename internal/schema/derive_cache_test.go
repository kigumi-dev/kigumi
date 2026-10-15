package schema_test

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/schema"
)

const deriveDisplaySrc = `
import fmt from std/fmt

type Point = {
    pub x Int
    pub y Int
}

pure fn render[T: fmt.Display](v: T) -> String {
    v.display()
}

print render(Point { x: 1, y: 2 })
`

func generatedOutputDigest(t *testing.T) string {
	t.Helper()
	h := sha256.New()
	for _, src := range []string{deriveParitySrc, deriveDisplaySrc} {
		m, _ := loadModule(t, src)
		a, err := driver.Analyze(m)
		if err != nil || a.Res.HasErrors() {
			t.Fatalf("analyze: %v", err)
		}
		var lines []string
		for _, g := range a.Res.GeneratedSources() {
			lines = append(lines, g.Package+"/"+g.Name+"\n"+g.Text)
		}
		sort.Strings(lines)
		for _, l := range lines {
			fmt.Fprintf(h, "%d\n%s\n", len(l), l)
		}
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func TestGeneratorVersionPinsOutput(t *testing.T) {
	got := fmt.Sprintf("version %d\ndigest %s\n", schema.GeneratorVersion, generatedOutputDigest(t))
	golden := "../../testdata/schema/generator_output.golden"
	if os.Getenv("UPDATE") == "1" {
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v (UPDATE=1 to create it)", err)
	}
	if string(want) != got {
		t.Errorf("derive generator output changed: bump schema.GeneratorVersion, then UPDATE=1\nwant:\n%sgot:\n%s", want, got)
	}
}

func TestDeriveCacheHitsAndRecoversFromCorruption(t *testing.T) {
	cache := &schema.DeriveCache{Dir: t.TempDir()}
	run := func() string {
		m, _ := loadModule(t, deriveParitySrc)
		a, err := driver.AnalyzeWith(m, cache.Generate)
		if err != nil || a.Res.HasErrors() {
			t.Fatalf("analyze: %v", err)
		}
		return fmt.Sprint(a.Res.GeneratedSources())
	}
	cold := run()
	misses := cache.Misses()
	if misses == 0 || cache.Hits() != 0 {
		t.Fatalf("cold run: %d hits, %d misses", cache.Hits(), misses)
	}
	if warm := run(); warm != cold || cache.Misses() != misses || cache.Hits() != misses {
		t.Fatalf("warm run: %d hits, %d misses (cold %d), same output %v", cache.Hits(), cache.Misses(), misses, warm == cold)
	}
	entries, _ := filepath.Glob(filepath.Join(cache.Dir, "*"))
	for _, e := range entries {
		data, _ := os.ReadFile(e)
		os.WriteFile(e, data[:len(data)-3], 0o644)
	}
	hits := cache.Hits()
	if again := run(); again != cold || cache.Hits() != hits || cache.Misses() != 2*misses {
		t.Fatalf("corrupt entries must be regenerated: %d hits, %d misses, same output %v", cache.Hits(), cache.Misses(), again == cold)
	}
}
