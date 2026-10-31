package driver

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	buildconfig "kigumi/internal/toolchain/config"
	"kigumi/internal/toolchain/version"
)

func TestPackageSCCsCollapseCyclesDependenciesFirst(t *testing.T) {
	deps := map[string][]string{"a": {"b"}, "b": {"a"}, "c": {"a"}, "d": nil}
	got := packageSCCs([]string{"c", "a", "d", "b"}, deps)
	want := [][]string{{"a", "b"}, {"c"}, {"d"}}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if !slices.Equal(got[i], want[i]) {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestCacheEnvCoversTargetAndFeatureFlags(t *testing.T) {
	base := func() *Module { return &Module{target: buildconfig.HostTarget()} }
	want := base().cacheEnv()
	if base().cacheEnv() != want {
		t.Fatal("the environment hash is deterministic")
	}
	variants := map[string]func(m *Module){
		"os":            func(m *Module) { m.target.OS = "freebsd" },
		"arch":          func(m *Module) { m.target.Arch = "arm64" },
		"allocator":     func(m *Module) { m.target.Allocator = "none" },
		"profile":       func(m *Module) { m.target.Profile = "recoverable-only" },
		"blocking":      func(m *Module) { m.target.BlockingExecutor = "isr" },
		"stack":         func(m *Module) { m.target.StrictStack = true },
		"deny":          func(m *Module) { m.deny = []string{"std/net"} },
		"ceiling":       func(m *Module) { m.ceiling = "core" },
		"artifact":      func(m *Module) { m.artifactName = "kernel" },
		"test":          func(m *Module) { m.test = true },
		"verify passes": func(m *Module) { m.verifyPasses = true },
	}
	for name, change := range variants {
		m := base()
		change(m)
		if m.cacheEnv() == want {
			t.Errorf("%s must change the environment hash", name)
		}
	}
}

func TestCacheEnvMovesWithCompilerAndRuntimeAbi(t *testing.T) {
	m := &Module{target: buildconfig.HostTarget()}
	base := m.cacheEnv()
	defer func(c func() version.Provenance, a func() string, e func() (string, error)) {
		compilerIdentity, runtimeAbi, executablePath = c, a, e
	}(compilerIdentity, runtimeAbi, executablePath)

	compilerIdentity = func() version.Provenance { p := version.Current(); p.Release += "-next"; return p }
	if m.cacheEnv() == base {
		t.Error("a compiler release must change the environment hash")
	}
	compilerIdentity = func() version.Provenance { p := version.Current(); p.Commit += "x"; return p }
	if m.cacheEnv() == base {
		t.Error("a compiler commit must change the environment hash")
	}
	compilerIdentity = version.Current

	runtimeAbi = func() string { return "another-abi" }
	if m.cacheEnv() == base {
		t.Error("a runtime ABI change must change the environment hash")
	}
	runtimeAbi = defaultRuntimeAbi

	dir := t.TempDir()
	exe := filepath.Join(dir, "kigumi")
	executablePath = func() (string, error) { return exe, nil }
	os.WriteFile(exe, []byte("one"), 0o755)
	first := m.cacheEnv()
	os.WriteFile(exe, []byte("another binary"), 0o755)
	if m.cacheEnv() == first {
		t.Error("a rebuilt compiler binary must change the environment hash")
	}
}

func TestPackageCachePrunesOldestOnceOverLimit(t *testing.T) {
	c := newPackageCache(t.TempDir())
	c.limit = 4000
	var digests []string
	for i := range 40 {
		d, _, ok := c.putAPI([]byte(strings.Repeat("x", 200) + fmt.Sprint(i)))
		if !ok {
			t.Fatal("write failed")
		}
		digests = append(digests, d)
		os.Chtimes(c.path("api", d), time.Now(), time.Now().Add(time.Duration(i)*time.Second-time.Minute))
	}
	c.prune()
	if c.stats.Pruned == 0 {
		t.Fatal("an over-limit store must prune")
	}
	var left int64
	filepath.WalkDir(filepath.Join(c.dir, "api"), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			st, _ := d.Info()
			left += st.Size()
		}
		return nil
	})
	if left > c.limit {
		t.Errorf("store is %d bytes after pruning, limit %d", left, c.limit)
	}
	if c.hasAPI(digests[0]) {
		t.Error("the oldest artifact must go first")
	}
	if !c.hasAPI(digests[len(digests)-1]) {
		t.Error("the newest artifact must stay")
	}
}

func TestPackageCacheKeepsStoreUnderLimit(t *testing.T) {
	c := newPackageCache(t.TempDir())
	c.limit = 6000
	for i := range pruneEvery*3 + 5 {
		c.putAPI([]byte(strings.Repeat("y", 300) + fmt.Sprint(i)))
	}
	if c.stats.Pruned == 0 {
		t.Error("a long run of writes must prune without an explicit call")
	}
}
