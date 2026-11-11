package driver_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/toolchain/driver"
)

func sessionModule(t *testing.T) *driver.Module {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"session\"\n    kigumi: \"0.1\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte("fn twice(n: Int) -> Int {\n    n * 2\n}\n\nprint \"${twice(21)}\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// Check, Analyze and further Analyze calls on an attached session share one
// check and one lowering, and the published analysis carries content ids.
func TestCompilationSessionSharesAnalysis(t *testing.T) {
	t.Parallel()
	m := sessionModule(t)
	s := driver.NewCompilationSession()
	m.AttachSession(s)
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if res.HasErrors() {
		t.Fatalf("unexpected errors:\n%s", res.Render())
	}
	if !res.Sealed() {
		t.Fatal("Check must publish (seal) the result")
	}
	afterCheck := m.Stats()
	if afterCheck.Check != 1 || afterCheck.MIR != 1 {
		t.Fatalf("one check and one lowering, got %+v", afterCheck)
	}
	first, err := driver.Analyze(m)
	if err != nil {
		t.Fatal(err)
	}
	if m.Stats() != afterCheck {
		t.Fatalf("Analyze rechecked a cached module: %+v -> %+v", afterCheck, m.Stats())
	}
	second, err := driver.Analyze(m)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || first.Res != res {
		t.Fatal("the session did not return its one shared analysis")
	}
	snap := first.Snapshot()
	if snap == nil || snap.ModuleID() == "" || snap.HasErrors() {
		t.Fatalf("published snapshot is not usable: %+v", snap)
	}
	if got := s.ParseSnapshot(m).ModuleID(); got != snap.ModuleID() {
		t.Fatalf("parse and analysis ids differ: %q != %q", got, snap.ModuleID())
	}
	if _, ok := snap.PackageID("session"); !ok {
		t.Fatalf("the root package has no content id: %+v", snap.Packages())
	}
	// A second check and a run share the published analysis without appending
	// to it (which would panic) or rechecking.
	var out, errOut bytes.Buffer
	code, err := s.Run(m, driver.RunOptions{}, &out, &errOut, nil)
	if err != nil || code != 0 {
		t.Fatalf("run on the shared analysis: code=%d err=%v stderr=%q", code, err, errOut.String())
	}
	if out.String() != "42\n" {
		t.Fatalf("run output = %q", out.String())
	}
	if m.Stats() != afterCheck {
		t.Fatalf("run rechecked a cached module: %+v", m.Stats())
	}
}

// Equal contents give equal ids across separately loaded modules.
func TestModuleIDIsContentDerived(t *testing.T) {
	t.Parallel()
	first := sessionModule(t)
	second := sessionModule(t)
	if first.ModuleID() != second.ModuleID() {
		t.Fatalf("equal contents got different module ids: %q != %q", first.ModuleID(), second.ModuleID())
	}
	a, ok := first.PackageID("session")
	b, ok2 := second.PackageID("session")
	if !ok || !ok2 || a == "" || a != b {
		t.Fatalf("equal packages got different ids: %q != %q", a, b)
	}
}

// A session must not hand back an analysis for changed bytes.
func TestCompilationSessionInvalidatesOnEdit(t *testing.T) {
	t.Parallel()
	m := sessionModule(t)
	s := driver.NewCompilationSession()
	m.AttachSession(s)
	if _, err := s.Analyze(m); err != nil {
		t.Fatal(err)
	}
	if m.Stats().Check != 1 {
		t.Fatalf("the first content was not checked: %+v", m.Stats())
	}
	std, _ := filepath.Abs("../../../std")
	m2, err := driver.LoadModule(m.Root, driver.LoadOptions{
		Compiler: testCompiler(),
		StdRoot:  std,
		Overlay:  map[string][]byte{filepath.Join(m.Root, "main.kg"): []byte("print \"changed\"\n")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if m2.ModuleID() == m.ModuleID() {
		t.Fatal("an edited file must change the module id")
	}
	if _, err := s.Analyze(m2); err != nil {
		t.Fatal(err)
	}
	if m2.Stats().Check != 1 {
		t.Fatalf("the new content must be checked, got %+v", m2.Stats())
	}
}
