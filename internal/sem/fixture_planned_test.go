package sem_test

import (
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/driver"
	"kigumi/internal/sem"
)

func checkPlanned(t *testing.T, fx fixture) *sem.Result {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "mod.kg"), []byte("Module {\n    name: \"fixture\"\n    kigumi: \"0.1\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, s := range fx.sections {
		switch s.name {
		case "module", "expect":
		case "src":
			os.WriteFile(filepath.Join(root, "main.kg"), []byte(s.body), 0o644)
		default:
			t.Fatalf("%s: a planned fixture holds only a src section", fx.path)
		}
	}
	std, _ := filepath.Abs("../../std")
	target := driver.HostTarget()
	target.Allocator = fx.opt("allocator", "")
	target.Profile = fx.opt("profile", "")
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler(), StdRoot: std, Target: target})
	if err != nil {
		t.Fatal(err)
	}
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	return res
}
