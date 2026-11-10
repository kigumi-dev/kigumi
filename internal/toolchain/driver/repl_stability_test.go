package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/driver"
)

// TestReplEntityIDsStayStable is the REPL session gate: appending a declaration
// must not renumber or retype the declarations a held value was compiled against.
func TestReplEntityIDsStayStable(t *testing.T) {
	root := t.TempDir()
	write := func(name, src string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("mod.kg", "Module {\n    name: \"probe\"\n    kigumi: \"0.1\"\n}\n")
	base := "fn first() -> Int {\n    1\n}\n\ntype A = {\n    pub x Int\n}\n"
	write("main.kg", base)
	opts := driver.LoadOptions{Entry: "main.kg", Compiler: testCompilerAt(root)}
	before := replEntities(t, root, opts)
	write("main.kg", base+"\nfn second() -> Int {\n    2\n}\n")
	after := replEntities(t, root, opts)
	for _, name := range []string{"first", "A", "x"} {
		if before[name] == "" || after[name] == "" {
			t.Fatalf("%s missing before=%q after=%q", name, before[name], after[name])
		}
		if before[name] != after[name] {
			t.Errorf("%s changed from %s to %s when a declaration was appended", name, before[name], after[name])
		}
	}
}

func replEntities(t *testing.T, root string, opts driver.LoadOptions) map[string]string {
	t.Helper()
	m, err := driver.LoadModule(root, opts)
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.CheckOnly(m)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for i := 1; i < len(res.Entities); i++ {
		e := res.Entity(sem.EntityID(i))
		if e == nil || e.Name == "" || e.Kind == sem.EntLocal || e.Kind == sem.EntParam {
			continue
		}
		// Type identity includes the entity id and the checked source spelling.
		out[e.Name] = strings.Join([]string{string(rune(e.Kind + 'a')), res.TypeString(e.Type), intString(int(e.Parent))}, ":")
	}
	return out
}

func intString(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

// TestTwoTopLevelStatementFiles is the F11 guard the REPL's synthetic entry
// design depends on: a non-entry file may not carry its own top-level script.
func TestTwoTopLevelStatementFiles(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"probe\"\n    kigumi: \"0.1\"\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("let a = 1\n"), 0o644)
	os.WriteFile(filepath.Join(root, "other.kg"), []byte("let b = 2\n"), 0o644)
	m, err := driver.LoadModule(root, driver.LoadOptions{Entry: "main.kg", Compiler: testCompilerAt(root)})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.CheckOnly(m)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HasErrors() || !strings.Contains(res.Render(), "E915") {
		t.Fatalf("two top-level statement files must be E915, got:\n%s", res.Render())
	}
}
