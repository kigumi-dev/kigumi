package driver_test

import (
	"strings"
	"testing"

	"kigumi/internal/driver"
)

func unitLoad(t *testing.T, root, cache, entry string) *driver.Module {
	t.Helper()
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: stdRoot(t), Entry: entry, PackageCache: cache, DeferParse: true, UnitCheck: true})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func unitCheck(t *testing.T, root, cache, entry string) (*driver.Module, string, bool) {
	t.Helper()
	m := unitLoad(t, root, cache, entry)
	res, err := driver.CheckUnits(m)
	if err != nil {
		t.Fatal(err)
	}
	return m, res.Render(), res.HasErrors()
}

func wholeCheck(t *testing.T, root, entry string) (string, bool) {
	t.Helper()
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: stdRoot(t), Entry: entry})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	return res.Render(), res.HasErrors()
}

func sameAsWhole(t *testing.T, root, cache, entry string) *driver.Module {
	t.Helper()
	m, got, gotErr := unitCheck(t, root, cache, entry)
	want, wantErr := wholeCheck(t, root, entry)
	if got != want || gotErr != wantErr {
		t.Fatalf("unit check differs from the whole check\n--- unit (errors=%v)\n%s--- whole (errors=%v)\n%s", gotErr, got, wantErr, want)
	}
	return m
}

func checkedPackages(s driver.UnitStats) []string {
	var out []string
	for p, n := range s.Checked {
		out = append(out, strings.TrimPrefix(p, "graphmod/")+"="+string(rune('0'+n)))
	}
	return out
}

func TestUnitPrivateEditChecksOnlyThatPackage(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	writeGraph(t, root, graphFiles())
	m := sameAsWhole(t, root, cache, "cmd/app/main.kg")
	if m.UnitStats().Checked["graphmod/leaf"] == 0 {
		t.Fatalf("a cold check checks every package: %v", checkedPackages(m.UnitStats()))
	}
	writeGraph(t, root, map[string]string{"leaf/leaf.kg": "pub fn value() -> Int {\n    1 + 0\n}\n"})
	m = sameAsWhole(t, root, cache, "cmd/app/main.kg")
	st := m.UnitStats()
	if st.Checked["graphmod/leaf"] != 1 || st.Checked["graphmod/mid"] != 0 || st.Checked["graphmod/cmd/app"] != 0 {
		t.Fatalf("a private edit re-checks only leaf: %v", checkedPackages(st))
	}
	if st.Replayed["graphmod/mid"] != 1 || st.Replayed["graphmod/cmd/app"] != 1 || st.Runs != 1 {
		t.Fatalf("the dependants replay from their clean records: %+v", st)
	}
	if m.Stats().Check != 1 {
		t.Fatalf("one semantic run: %+v", m.Stats())
	}
	if st.Lazy != 1 || st.LazyRetries != 0 || st.SkippedBodies == 0 {
		t.Fatalf("the run checks only the import bodies leaf names: %+v", st)
	}
}

func TestUnitNoopWritesNothing(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	writeGraph(t, root, graphFiles())
	sameAsWhole(t, root, cache, "cmd/app/main.kg")
	m, got, _ := unitCheck(t, root, cache, "cmd/app/main.kg")
	if got != "" || m.Stats().Check != 0 {
		t.Fatalf("an unchanged module answers from the settled record, got %q %+v", got, m.Stats())
	}
	if cs := m.PackageCacheStats(); cs.ArtifactWrites != 0 || cs.RecordWrites != 0 {
		t.Fatalf("a no-op check writes nothing: %+v", cs)
	}
}

func TestUnitPublicEditChecksReverseDependenciesOnly(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	files := graphFiles()
	files["cmd/app/main.kg"] = "import mid from graphmod/mid\nimport other from graphmod/other\n\nprint \"${mid.twice() + other.seven()}\"\n"
	writeGraph(t, root, files)
	sameAsWhole(t, root, cache, "cmd/app/main.kg")
	writeGraph(t, root, map[string]string{"leaf/leaf.kg": "pub fn value() -> Int {\n    1\n}\n\npub fn extra() -> Int {\n    2\n}\n"})
	m := sameAsWhole(t, root, cache, "cmd/app/main.kg")
	st := m.UnitStats()
	if st.Checked["graphmod/other"] != 0 || st.Replayed["graphmod/other"] != 1 {
		t.Fatalf("other does not import leaf and replays: %+v", st)
	}
	for _, p := range []string{"leaf", "mid", "cmd/app"} {
		if st.Checked["graphmod/"+p] == 0 {
			t.Fatalf("%s depends on leaf's changed API: %v", p, checkedPackages(st))
		}
	}
}

func TestUnitErrorsAreReportedAndNeverReplayed(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	writeGraph(t, root, graphFiles())
	sameAsWhole(t, root, cache, "cmd/app/main.kg")
	writeGraph(t, root, map[string]string{"mid/mid.kg": "import leaf from graphmod/leaf\n\npub fn twice() -> Int {\n    leaf.value() * \"x\"\n}\n"})
	m, got, gotErr := unitCheck(t, root, cache, "cmd/app/main.kg")
	want, wantErr := wholeCheck(t, root, "cmd/app/main.kg")
	if got != want || !gotErr || !wantErr {
		t.Fatalf("errors must match the whole check\n%s\n---\n%s", got, want)
	}
	if m.UnitStats().Checked["graphmod/mid"] == 0 {
		t.Fatalf("mid has an error and is checked: %+v", m.UnitStats())
	}
	m, got, _ = unitCheck(t, root, cache, "cmd/app/main.kg")
	if got != want || m.UnitStats().Replayed["graphmod/mid"] != 0 {
		t.Fatalf("an erroring package is checked again, never replayed: %+v", m.UnitStats())
	}
	writeGraph(t, root, graphFiles())
	sameAsWhole(t, root, cache, "cmd/app/main.kg")
}

func TestUnitWarningPackageIsCheckedEveryTime(t *testing.T) {
	t.Parallel()
	root, cache := t.TempDir(), t.TempDir()
	files := graphFiles()
	files["leaf/leaf.kg"] = "import other from graphmod/other\n\npub fn value() -> Int {\n    1\n}\n"
	writeGraph(t, root, files)
	sameAsWhole(t, root, cache, "cmd/app/main.kg")
	writeGraph(t, root, map[string]string{"cmd/app/main.kg": "import mid from graphmod/mid\n\nprint \"${mid.twice() + 1}\"\n"})
	m := sameAsWhole(t, root, cache, "cmd/app/main.kg")
	st := m.UnitStats()
	if st.Replayed["graphmod/leaf"] != 0 || st.Checked["graphmod/leaf"] == 0 {
		t.Fatalf("a package that prints a warning is checked every time: %+v", st)
	}
	if st.Checked["graphmod/mid"] != 0 || st.Replayed["graphmod/mid"] != 1 {
		t.Fatalf("mid is clean and unchanged: %+v", st)
	}
}
