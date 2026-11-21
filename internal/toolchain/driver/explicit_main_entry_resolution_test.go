package driver_test

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
)

// TestRunExplicitMainNoExplicitEntry covers `kigumi run <dir>` (no filename
// argument): Module.defaultEntry must recognize an explicit
// `fn main() -> Void!` root file as the entry, not only a script-shaped one.
func TestRunExplicitMainNoExplicitEntry(t *testing.T) {
	root := t.TempDir()
	writeEntryScopeModule(t, root, map[string]string{
		"main.kg": "fn main() -> Void! {\n    print(\"hello\")\n}\n",
	})
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	var out, errOut bytes.Buffer
	code, err := driver.RunWith(m, driver.RunOptions{}, &out, &errOut, nil)
	if err != nil {
		t.Fatalf("%v\n%s", err, errOut.String())
	}
	if out.String() != "hello\n" {
		t.Errorf("stdout = %q, want %q (stderr: %s)", out.String(), "hello\n", errOut.String())
	}
	if code != 0 {
		t.Errorf("exit = %d, want 0\nstderr:\n%s", code, errOut.String())
	}
}

// TestDefaultEntryPrefersMainKg covers `kigumi run <dir>` with main.kg and a
// sibling script: main.kg is the conventional entry even when both files have
// top-level statements.
func TestDefaultEntryPrefersMainKg(t *testing.T) {
	root := t.TempDir()
	writeEntryScopeModule(t, root, map[string]string{
		"main.kg":  "print \"main\"\n",
		"other.kg": "print \"other\"\n",
	})
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if m.Entry != "main.kg" {
		t.Fatalf("Entry = %q, want main.kg", m.Entry)
	}
	out := res.Render()
	if !strings.Contains(out, "--> other.kg") || strings.Contains(out, "--> main.kg") {
		t.Fatalf("only the sibling script should be stray, got:\n%s", out)
	}
}

// TestCheckExplicitMainCmdPackage covers `kigumi check` on a whole
// build-program-shaped module (no explicit entry): Module.semModule's cmd/
// loop must give a cmd/ package written as an explicit `fn main` its own
// entry too, not only script-shaped ones.
func TestCheckExplicitMainCmdPackage(t *testing.T) {
	root := t.TempDir()
	writeEntryScopeModule(t, root, map[string]string{
		"mod.kg":               "Module {\n    name: \"cmdmain\"\n    kigumi: \"0.1\"\n}\n",
		"cmd/script/main.kg":   "print \"from script\"\n",
		"cmd/explicit/main.kg": "fn main() -> Void! {\n    print(\"from explicit\")\n}\n",
	})
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if res.HasErrors() {
		t.Fatalf("both cmd/ entry styles should check clean:\n%s", res.Render())
	}
}

func TestExplicitEntryDoesNotPromoteSiblingCmdScript(t *testing.T) {
	root := t.TempDir()
	writeEntryScopeModule(t, root, map[string]string{
		"cmd/app/main.kg":  "print \"main\"\n",
		"cmd/app/other.kg": "print \"other\"\n",
	})
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Entry: "cmd/app/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	out := res.Render()
	if strings.Contains(out, "two script files") || strings.Contains(out, "--> cmd/app/main.kg") || !strings.Contains(out, "--> cmd/app/other.kg") {
		t.Fatalf("explicit entry should leave only the sibling stray, got:\n%s", out)
	}
}

func TestExplicitMainInSelectedEntryWins(t *testing.T) {
	root := t.TempDir()
	writeEntryScopeModule(t, root, map[string]string{
		"mod.kg":       "Module {\n    name: \"m\"\n    kigumi: \"0.1\"\n}\n",
		"aaa/lib.kg":   "fn main() -> Void { print \"aaa\" }\n",
		"main/main.kg": "fn main() -> Void { print \"main\" }\n",
	})
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Entry: "main/main.kg"})
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())
	var out, errOut bytes.Buffer
	code, err := driver.RunWith(m, driver.RunOptions{}, &out, &errOut, nil)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 || out.String() != "main\n" {
		t.Fatalf("selected explicit main = %q, exit %d, stderr %s", out.String(), code, errOut.String())
	}
}

func TestDefaultEntryIgnoresMainPackageUnderManifest(t *testing.T) {
	root := t.TempDir()
	writeEntryScopeModule(t, root, map[string]string{
		"mod.kg":       "Module {\n    name: \"m\"\n    kigumi: \"0.1\"\n}\n",
		"app.kg":       "print \"app\"\n",
		"main/main.kg": "print \"pkg\"\n",
	})
	std, _ := filepath.Abs("../../../std")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if m.Entry != "app.kg" {
		t.Fatalf("Entry = %q, want app.kg", m.Entry)
	}
	if strings.Contains(res.Render(), "--> app.kg") || !strings.Contains(res.Render(), "--> main/main.kg") {
		t.Fatalf("root script should be the only stray, got:\n%s", res.Render())
	}
}
