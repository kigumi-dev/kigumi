package arch_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

var shardProbeFiles = map[string]string{
	"internal/devkit/fixture/shard.go": `package fixture

func Shard(id string) bool { return id != "" }

func Each(ids []string, f func(string)) {
	for _, id := range ids {
		if Shard(id) {
			f(id)
		}
	}
}
`,
	"internal/devkit/fixture/other.go": "package fixture\n\nfunc Count(ids []string) int { return len(ids) }\n",
	"probe/helper_internal_test.go": `package probe

import "kigumi/internal/devkit/fixture"

type corpus struct{ ids []string }

func (c corpus) walk(f func(string)) { fixture.Each(c.ids, f) }

func EachProbe(ids []string, f func(string)) {
	for _, id := range ids {
		if fixture.Shard(id) {
			f(id)
		}
	}
}
`,
	"probe/probe_test.go": `package probe_test

import (
	"testing"

	"kigumi/internal/devkit/fixture"
	"kigumi/probe"
)

func TestDirect(t *testing.T)   { _ = fixture.Shard("a") }
func TestDevkit(t *testing.T)   { fixture.Each(nil, func(string) {}) }
func TestIndirect(t *testing.T) { probe.EachProbe(nil, func(string) {}) }
func TestPlain(t *testing.T)    { _ = fixture.Count(nil) }
`,
	"probe/method_internal_test.go": `package probe

import "testing"

func TestMethod(t *testing.T) { corpus{}.walk(func(string) {}) }
`,
}

// The scan follows a shard call through a devkit helper, an exported helper of the package's own
// internal test files and a method, and leaves a test that only uses other devkit calls out.
func TestShardCallersFollowHelpersAcrossPackages(t *testing.T) {
	root := t.TempDir()
	for name, src := range shardProbeFiles {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := shardCallers(t, root, filepath.Join(root, "probe"), "kigumi/probe")
	if want := []string{"TestDevkit", "TestDirect", "TestIndirect", "TestMethod"}; !slices.Equal(got, want) {
		t.Errorf("shard callers = %v, want %v", got, want)
	}
}
