package driver

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeUnitFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	old := time.Now().Add(-time.Hour)
	for rel, body := range files {
		full := filepath.Join(root, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(full), 0o755)
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		os.Chtimes(full, old, old)
	}
}

// A clean record that turns out to be wrong is withdrawn and the whole module is checked, so the stale record cannot hide the diagnostic.
func TestUnitDemotesAWronglyCleanPackage(t *testing.T) {
	std, _ := filepath.Abs("../../std")
	root, cache := t.TempDir(), t.TempDir()
	files := map[string]string{
		"mod.kg":          "Module {\n    name: \"demote\"\n    kigumi: \"0.1\"\n}\n",
		"other/other.kg":  "pub fn seven() -> Int {\n    7\n}\n",
		"lib/lib.kg":      "import other from demote/other\n\npub fn value() -> Int {\n    1\n}\n",
		"cmd/app/main.kg": "import lib from demote/lib\n\nprint \"${lib.value()}\"\n",
	}
	writeUnitFiles(t, root, files)
	load := func() *Module {
		m, err := LoadModule(root, LoadOptions{StdRoot: std, Entry: "cmd/app/main.kg", PackageCache: cache, DeferParse: true, UnitCheck: true})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	first := load()
	res, err := CheckUnits(first)
	if err != nil || !strings.Contains(res.Render(), "unused import") {
		t.Fatalf("lib has an unused import: %v\n%s", err, res.Render())
	}
	var lib PackageStatus
	for _, s := range first.PackageStatuses() {
		if s.Path == "demote/lib" {
			lib = s
		}
	}
	if lib.Clean {
		t.Fatal("a package with a warning is not clean")
	}
	first.pkgCache.putSealed("pkg", lib.Key, keyRecord{Path: lib.Path, Digest: lib.Digest, Clean: true})
	writeUnitFiles(t, root, map[string]string{"cmd/app/main.kg": "import lib from demote/lib\n\nprint \"${lib.value() + 1}\"\n"})
	second := load()
	res, err = CheckUnits(second)
	if err != nil || !strings.Contains(res.Render(), "unused import") {
		t.Fatalf("the warning must survive a wrong clean record: %v\n%s", err, res.Render())
	}
	if st := second.UnitStats(); st.Demoted != 1 || st.Fallbacks != 1 {
		t.Fatalf("the record is withdrawn and the module checked whole: %+v", st)
	}
}
