package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/driver"
)

const isrDepLib = `export(interrupt) {
    pub fn tick() {}
}
`

func isrDepModule(t *testing.T, rootManifest, depManifest string) *driver.Module {
	t.Helper()
	dir := t.TempDir()
	lib, app := filepath.Join(dir, "lib"), filepath.Join(dir, "app")
	os.MkdirAll(lib, 0o755)
	os.MkdirAll(app, 0o755)
	os.WriteFile(filepath.Join(lib, "mod.kg"), []byte("Module {\n    name: \"lib\"\n    kigumi: \"0.1\"\n}\n"+depManifest), 0o644)
	os.WriteFile(filepath.Join(lib, "lib.kg"), []byte(isrDepLib), 0o644)
	os.WriteFile(filepath.Join(app, "mod.kg"), []byte("Module {\n    name: \"app\"\n    kigumi: \"0.1\"\n}\nRequire { name: \"lib\", version: \"v1.0.0\", url: \"https://example.invalid/lib.git\" }\n"+rootManifest), 0o644)
	os.WriteFile(filepath.Join(app, "mod.local.kg"), []byte("Replace { name: \"lib\", path: \"../lib\" }\n"), 0o644)
	os.WriteFile(filepath.Join(app, "main.kg"), []byte("import {tick} from lib\n\nfn main() -> Void! {\n    tick()\n}\n"), 0o644)
	std, _ := filepath.Abs("../../std")
	m, err := driver.LoadModule(app, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func checkRender(t *testing.T, m *driver.Module) string {
	t.Helper()
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	return res.Render()
}

func TestInterruptLoweringDependencyRecordMerged(t *testing.T) {
	t.Parallel()
	m := isrDepModule(t, "Interrupt { allow: \"\" }\n", "Interrupt { handler: \"lib.tick\", allow: \"\" }\n")
	if out := checkRender(t, m); strings.Contains(out, "error") {
		t.Fatalf("a dependency record inside the root limit must be accepted:\n%s", out)
	}
}

func TestInterruptLoweringDependencyRecordRejected(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, dep, code, want string }{
		{"outside the root limit", "Interrupt { handler: \"lib.tick\", allow: \"sync q.n width=32 impl=cas\" }\n", "E1091", "the `Interrupt` record of dependency `lib` for handler `lib.tick` names `sync q.n width=32 impl=cas`"},
		{"foreign handler", "Interrupt { handler: \"app.tick\", allow: \"\" }\n", "E2141", "names handler `app.tick`, which belongs to another module"},
	}
	for _, c := range cases {
		out := checkRender(t, isrDepModule(t, "Interrupt { allow: \"\" }\n", c.dep))
		if !strings.Contains(out, c.code) || !strings.Contains(out, c.want) {
			t.Fatalf("%s: the dependency record must be rejected with %s and %q:\n%s", c.name, c.code, c.want, out)
		}
	}
}

func TestInterruptLoweringDependencyCommonRecordIgnored(t *testing.T) {
	t.Parallel()
	for _, dep := range []string{"Interrupt { allow: \"\" }\n", "Interrupt { allow: \"sync q.n width=32 impl=cas\" }\n"} {
		if out := checkRender(t, isrDepModule(t, "Interrupt { allow: \"\" }\n", dep)); strings.Contains(out, "error") {
			t.Fatalf("a dependency's common record must not affect the consumer (%q):\n%s", dep, out)
		}
	}
}

func TestInterruptLoweringMissingHandlerEntry(t *testing.T) {
	t.Parallel()
	out := checkRender(t, isrDepModule(t, "Interrupt { handler: \"lib.gone\", allow: \"sync q.n width=32 impl=cas\" }\n", ""))
	for _, want := range []string{"E1091", "the `Interrupt` record for handler `lib.gone` names `sync q.n width=32 impl=cas`, which matches no reviewed contract", "E1092", "names handler `lib.gone`"} {
		if !strings.Contains(out, want) {
			t.Fatalf("an entry for a missing handler must be checked (missing %q):\n%s", want, out)
		}
	}
}
