package mod_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

// writeRenameModule lays out a module that imports its own package and a
// dependency, plus a comment and a string that merely mention the paths.
func writeRenameModule(t *testing.T, root string) string {
	t.Helper()
	app := filepath.Join(root, "app")
	os.MkdirAll(filepath.Join(app, "sub"), 0o755)
	os.MkdirAll(filepath.Join(root, "dep"), 0o755)
	os.WriteFile(filepath.Join(app, "mod.kg"), clitest.Manifest("github.com/acme/app",
		"Require {\n    name: \"github.com/acme/dep\"\n    version: \"v1.0.0\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(app, "mod.local.kg"),
		[]byte("Replace { name: \"github.com/acme/dep\", path: \"../dep\" }\n"), 0o644)
	os.WriteFile(filepath.Join(app, "sub", "sub.kg"), []byte("pub fn one() -> Int {\n    1\n}\n"), 0o644)
	os.WriteFile(filepath.Join(app, "main.kg"), []byte(`// github.com/acme/dep appears in this comment and must stay
import {one} from github.com/acme/app/sub
import {two} from github.com/acme/dep
print "${one()} ${two()}"
print "github.com/acme/dep"
`), 0o644)
	return app
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// One argument renames this module: the Module record and imports of its own
// packages change, while Requires, comments and strings stay untouched.
func TestModRenameCurrentModule(t *testing.T) {
	app := writeRenameModule(t, t.TempDir())
	if code, _, errOut := clitest.Execute(t, "mod", "rename", "-C", app, "github.com/neworg/app"); code != 0 {
		t.Fatalf("rename: %s", errOut)
	}
	if mf := readFile(t, filepath.Join(app, "mod.kg")); !strings.Contains(mf, "name: \"github.com/neworg/app\"") {
		t.Fatalf("manifest was not renamed:\n%s", mf)
	}
	main := readFile(t, filepath.Join(app, "main.kg"))
	if !strings.Contains(main, "import {one} from github.com/neworg/app/sub") {
		t.Fatalf("self import was not rewritten:\n%s", main)
	}
	if !strings.Contains(main, "import {two} from github.com/acme/dep") {
		t.Fatalf("a dependency import must not change:\n%s", main)
	}
	if !strings.Contains(main, "// github.com/acme/dep appears in this comment and must stay") ||
		!strings.Contains(main, `print "github.com/acme/dep"`) {
		t.Fatalf("comments and strings must not be rewritten:\n%s", main)
	}
	if local := readFile(t, filepath.Join(app, "mod.local.kg")); !strings.Contains(local, "github.com/acme/dep") {
		t.Fatalf("an unrelated Replace must not change:\n%s", local)
	}
}

// Two arguments rename a required module everywhere it is named: the
// Require, its Replace and every import, but never mod.lock.kg.
func TestModRenameRequiredModule(t *testing.T) {
	app := writeRenameModule(t, t.TempDir())
	os.WriteFile(filepath.Join(app, "mod.lock.kg"), []byte("Lock {\n    name: \"github.com/acme/dep\"\n    version: \"v1.0.0\"\n    commit: \"sha1:0000000000000000000000000000000000000000\"\n    hash: \"h1:00\"\n}\n"), 0o644)
	code, _, errOut := clitest.Execute(t, "mod", "rename", "-C", app, "github.com/acme/dep", "example.com/moved/dep")
	if code != 0 {
		t.Fatalf("rename: %s", errOut)
	}
	if !strings.Contains(errOut, "mod.lock.kg still records github.com/acme/dep") {
		t.Fatalf("the lock must be reported as stale, not silently rewritten: %q", errOut)
	}
	if mf := readFile(t, filepath.Join(app, "mod.kg")); !strings.Contains(mf, "name: \"example.com/moved/dep\"") {
		t.Fatalf("Require was not renamed:\n%s", mf)
	}
	if main := readFile(t, filepath.Join(app, "main.kg")); !strings.Contains(main, "import {two} from example.com/moved/dep") {
		t.Fatalf("import was not renamed:\n%s", main)
	}
	if local := readFile(t, filepath.Join(app, "mod.local.kg")); !strings.Contains(local, "name: \"example.com/moved/dep\"") {
		t.Fatalf("Replace was not renamed:\n%s", local)
	}
	if lock := readFile(t, filepath.Join(app, "mod.lock.kg")); !strings.Contains(lock, "github.com/acme/dep") {
		t.Fatalf("the lock must not be rewritten:\n%s", lock)
	}
}

// A rename never merges two modules, and a major suffix may only take
// versions of that major.
func TestModRenameRefusals(t *testing.T) {
	app := writeRenameModule(t, t.TempDir())
	os.WriteFile(filepath.Join(app, "mod.kg"), clitest.Manifest("github.com/acme/app",
		"Require {\n    name: \"github.com/acme/dep\"\n    version: \"v1.0.0\"\n}\nRequire {\n    name: \"github.com/acme/other\"\n    version: \"v1.0.0\"\n}\n"), 0o644)
	if code, _, errOut := clitest.Execute(t, "mod", "rename", "-C", app, "github.com/acme/dep", "github.com/acme/other"); code != 2 || !strings.Contains(errOut, "already required") {
		t.Fatalf("merging two modules must be refused, exit %d: %q", code, errOut)
	}
	if code, _, errOut := clitest.Execute(t, "mod", "rename", "-C", app, "github.com/acme/dep", "example.com/moved/dep/v2"); code != 2 || !strings.Contains(errOut, "major suffix") {
		t.Fatalf("a v1 Require cannot move to a /v2 path, exit %d: %q", code, errOut)
	}
	if code, _, errOut := clitest.Execute(t, "mod", "rename", "-C", app, "github.com/acme/dep", "github.com/acme/dep"); code != 2 {
		t.Fatalf("a no-op rename must be refused, exit %d: %q", code, errOut)
	}
	if mf := readFile(t, filepath.Join(app, "mod.kg")); !strings.Contains(mf, "name: \"github.com/acme/dep\"") {
		t.Fatalf("a refused rename must leave the manifest alone:\n%s", mf)
	}
}
