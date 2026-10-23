package driver_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"kigumi/internal/driver"
)

const graphMod = "Module {\n    name: \"graphmod\"\n    kigumi: \"0.1\"\n}\n"

func graphFiles() map[string]string {
	return map[string]string{
		"mod.kg":           graphMod,
		"leaf/leaf.kg":     "pub fn value() -> Int {\n    1\n}\n",
		"mid/mid.kg":       "import leaf from graphmod/leaf\n\npub fn twice() -> Int {\n    leaf.value() * 2\n}\n",
		"other/other.kg":   "pub fn seven() -> Int {\n    7\n}\n",
		"broken/broken.kg": "pub fn oops( {\n",
		"cmd/app/main.kg":  "import mid from graphmod/mid\n\nprint \"${mid.twice()}\"\n",
	}
}

func writeGraph(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	ageTree(t, root)
}

// ageTree moves every mtime out of the memo's racy window, as a file edited long ago would be.
func ageTree(t *testing.T, root string) {
	t.Helper()
	old := time.Now().Add(-time.Hour)
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			os.Chtimes(p, old, old)
		}
		return nil
	})
}

func stdRoot(t *testing.T) string {
	t.Helper()
	std, err := filepath.Abs("../../std")
	if err != nil {
		t.Fatal(err)
	}
	return std
}

func TestIndexParsesOnlyReachablePackages(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeGraph(t, root, graphFiles())
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), Entry: "cmd/app/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Order, []string{"graphmod/cmd/app", "graphmod/leaf", "graphmod/mid"}) {
		t.Fatalf("an entry loads its import closure only, got %v", m.Order)
	}
	if st := m.IndexStats(); st.PackagesIndexed != 5 || st.PackagesParsed != 3 {
		t.Fatalf("want 5 indexed and 3 parsed packages, got %+v", st)
	}
	if d := m.Diagnostics(); d != "" {
		t.Fatalf("the unreachable broken package must not report: %s", d)
	}
	whole, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler()})
	if err != nil {
		t.Fatal(err)
	}
	if len(whole.Order) != 5 || !strings.Contains(whole.Diagnostics(), "broken/broken.kg") {
		t.Fatalf("a load with no entry makes every package a root, got %v", whole.Order)
	}
}

func TestIndexRootDirsAndImports(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeGraph(t, root, graphFiles())
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), RootDirs: []string{"cmd/app"}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Order, []string{"graphmod/cmd/app", "graphmod/leaf", "graphmod/mid"}) {
		t.Fatalf("RootDirs roots the packages under the directory and what they import, got %v", m.Order)
	}
	m, err = driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), RootDirs: []string{}, RootImports: []string{"graphmod/other"}})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(m.Order, []string{"graphmod/other"}) {
		t.Fatalf("RootImports names roots by import path, got %v", m.Order)
	}
}

func TestBrokenUnreachablePackageDoesNotStopCheck(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeGraph(t, root, graphFiles())
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: stdRoot(t), Entry: "cmd/app/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if res.HasErrors() || m.HasParseErrors() {
		t.Fatalf("the entry does not reach the broken package:\n%s%s", m.Diagnostics(), res.Render())
	}
}

func TestDeferredLoadParsesNothing(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeGraph(t, root, graphFiles())
	cache := t.TempDir()
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: stdRoot(t), Entry: "cmd/app/main.kg", PackageCache: cache})
	if err != nil {
		t.Fatal(err)
	}
	if res, err := driver.Check(m); err != nil || res.HasErrors() {
		t.Fatalf("first check: %v\n%s", err, res.Render())
	}
	first := m.PackageCacheStats()
	if first.ArtifactWrites != 3 || first.SettledWrites != 1 {
		t.Fatalf("first check should write 3 artifacts and settle once, got %+v", first)
	}
	again, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: stdRoot(t), Entry: "cmd/app/main.kg", PackageCache: cache, DeferParse: true})
	if err != nil {
		t.Fatal(err)
	}
	if res, err := driver.Check(again); err != nil || res.HasErrors() {
		t.Fatalf("settled check: %v", err)
	}
	st, cs := again.IndexStats(), again.PackageCacheStats()
	if st.FilesRead != 0 || st.PackagesParsed != 0 || again.Stats().Check != 0 {
		t.Fatalf("a no-op load must read, parse and check nothing, got %+v %+v", st, again.Stats())
	}
	if cs.ArtifactWrites != 0 || cs.RecordWrites != 0 || cs.SettledHits != 1 {
		t.Fatalf("a no-op load regenerates no artifact, got %+v", cs)
	}
}

func recordStatuses(t *testing.T, root, cache string, entry string) map[string]driver.PackageStatus {
	t.Helper()
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: stdRoot(t), Entry: entry, PackageCache: cache})
	if err != nil {
		t.Fatal(err)
	}
	if res, err := driver.Check(m); err != nil || res.HasErrors() {
		t.Fatalf("check: %v\n%s", err, res.Render())
	}
	out := map[string]driver.PackageStatus{}
	for _, s := range m.PackageStatuses() {
		out[strings.TrimPrefix(s.Path, "graphmod/")] = s
	}
	return out
}

func TestPrivateEditKeepsDependantsArtifacts(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	writeGraph(t, root, graphFiles())
	recordStatuses(t, root, cache, "cmd/app/main.kg")
	writeGraph(t, root, map[string]string{"leaf/leaf.kg": "pub fn value() -> Int {\n    1 + 0\n}\n"})
	got := recordStatuses(t, root, cache, "cmd/app/main.kg")
	if got["leaf"].Hit {
		t.Fatalf("leaf must miss after its source changed: %+v", got)
	}
	if got["leaf"].Written {
		t.Fatalf("a private body edit leaves the API artifact byte-identical, so no new artifact: %+v", got["leaf"])
	}
	for _, p := range []string{"mid", "cmd/app"} {
		if !got[p].Hit {
			t.Fatalf("%s keys on leaf's artifact, which did not change: %+v", p, got[p])
		}
	}
}

func TestPublicEditInvalidatesReverseDependencies(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	files := graphFiles()
	files["cmd/app/main.kg"] = "import mid from graphmod/mid\nimport other from graphmod/other\n\nprint \"${mid.twice() + other.seven()}\"\n"
	writeGraph(t, root, files)
	recordStatuses(t, root, cache, "cmd/app/main.kg")
	writeGraph(t, root, map[string]string{"leaf/leaf.kg": "pub fn value() -> Int {\n    1\n}\n\npub fn extra() -> Int {\n    2\n}\n"})
	got := recordStatuses(t, root, cache, "cmd/app/main.kg")
	for _, p := range []string{"leaf", "mid", "cmd/app"} {
		if got[p].Hit {
			t.Fatalf("%s depends on leaf and must be invalidated: %+v", p, got[p])
		}
	}
	if !got["other"].Hit {
		t.Fatalf("other does not depend on leaf: %+v", got["other"])
	}
	if !got["leaf"].Written || got["mid"].Written || got["cmd/app"].Written {
		t.Fatalf("only leaf's API body is new: %+v", got)
	}
}

func TestCorruptCacheEntriesAreMisses(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	writeGraph(t, root, graphFiles())
	recordStatuses(t, root, cache, "cmd/app/main.kg")
	var damaged int
	filepath.WalkDir(cache, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && (strings.Contains(p, "/pkg/") || strings.Contains(p, "/api/") || strings.Contains(p, "/check/")) {
			os.WriteFile(p, []byte("not json"), 0o644)
			damaged++
		}
		return nil
	})
	if damaged == 0 {
		t.Fatal("no cache entries were written")
	}
	got := recordStatuses(t, root, cache, "cmd/app/main.kg")
	for p, s := range got {
		if s.Hit || !s.Written {
			t.Fatalf("%s: a corrupt entry is a miss and is rewritten: %+v", p, s)
		}
	}
}

func TestCacheKeyCoversTargetAndFeatureFlags(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	writeGraph(t, root, graphFiles())
	recordStatuses(t, root, cache, "cmd/app/main.kg")
	other, err := driver.ParseTarget(testCompiler(), "aarch64-linux", "")
	if err != nil {
		t.Fatal(err)
	}
	for name, opts := range map[string]driver.LoadOptions{
		"target":  {Target: other},
		"ceiling": {Ceiling: "core"},
		"deny":    {Deny: []string{"std/net"}},
	} {
		opts.Compiler, opts.StdRoot, opts.Entry, opts.PackageCache = testCompiler(), stdRoot(t), "cmd/app/main.kg", cache
		m, err := driver.LoadModule(root, opts)
		if err != nil {
			t.Fatal(err)
		}
		driver.CheckOnly(m)
		for _, s := range m.PackageStatuses() {
			if s.Hit {
				t.Fatalf("%s changes every key, but %s hit", name, s.Path)
			}
		}
	}
}

func TestEditorPathSharesIndexAndCache(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	files := graphFiles()
	delete(files, "broken/broken.kg")
	writeGraph(t, root, files)
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), Test: true, StdRoot: stdRoot(t), PackageCache: cache, AllPackages: true})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.CheckForEditor(m)
	if err != nil || res.HasErrors() {
		t.Fatalf("editor check: %v", err)
	}
	if got := len(m.PackageStatuses()); got != 4 {
		t.Fatalf("every package is a root in the editor and gets a record, got %d", got)
	}
	if st := m.PackageCacheStats(); st.ArtifactWrites != 4 {
		t.Fatalf("want 4 artifacts, got %+v", st)
	}
}

func TestEditedModuleIsNotSettled(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	writeGraph(t, root, graphFiles())
	recordStatuses(t, root, cache, "cmd/app/main.kg")
	writeGraph(t, root, map[string]string{"mid/mid.kg": "import leaf from graphmod/leaf\n\npub fn twice() -> Int {\n    leaf.value() + leaf.value()\n}\n"})
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: stdRoot(t), Entry: "cmd/app/main.kg", PackageCache: cache, DeferParse: true})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil || res.HasErrors() {
		t.Fatalf("check: %v", err)
	}
	if st := m.PackageCacheStats(); st.SettledHits != 0 || m.Stats().Check != 1 || m.IndexStats().PackagesParsed == 0 {
		t.Fatalf("an edited module is checked again: %+v %+v %+v", st, m.Stats(), m.IndexStats())
	}
	if !slices.ContainsFunc(m.PackageStatuses(), func(s driver.PackageStatus) bool { return s.Path == "graphmod/leaf" && s.Hit }) {
		t.Fatalf("leaf is untouched and keeps its record: %+v", m.PackageStatuses())
	}
}

func TestUnreachableEditDoesNotUnsettle(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	writeGraph(t, root, graphFiles())
	recordStatuses(t, root, cache, "cmd/app/main.kg")
	writeGraph(t, root, map[string]string{"broken/broken.kg": "pub fn still_broken( {\n"})
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: stdRoot(t), Entry: "cmd/app/main.kg", PackageCache: cache, DeferParse: true})
	if err != nil {
		t.Fatal(err)
	}
	if res, err := driver.Check(m); err != nil || res.HasErrors() || m.PackageCacheStats().SettledHits != 1 {
		t.Fatalf("a package the entry never reaches does not matter: %v %+v", err, m.PackageCacheStats())
	}
}

func TestMultiLineBraceImportKeysItsPackage(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	files := graphFiles()
	files["cmd/app/main.kg"] = "import {\n    twice\n} from graphmod/mid\n\nprint \"${twice()}\"\n"
	writeGraph(t, root, files)
	recordStatuses(t, root, cache, "cmd/app/main.kg")
	writeGraph(t, root, map[string]string{"leaf/leaf.kg": "pub fn value( {\n"})
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: stdRoot(t), Entry: "cmd/app/main.kg", PackageCache: cache, DeferParse: true})
	if err != nil {
		t.Fatal(err)
	}
	if res, err := driver.Check(m); err == nil && !res.HasErrors() {
		t.Fatalf("breaking a package behind a multi-line import must not replay a clean outcome: %+v", m.PackageCacheStats())
	}
}
