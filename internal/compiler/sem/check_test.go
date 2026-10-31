package sem_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/diag"
	"kigumi/internal/toolchain/contract/schema"
)

// TestSemCorpus runs every testdata/sem fixture against its annotations and
// its -- expect -- section; UPDATE=1 rewrites the expect section.
func TestSemCorpus(t *testing.T) {
	files, err := filepath.Glob("../../../testdata/sem/*.txtar")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("no fixtures under testdata/sem")
	}
	for _, path := range files {
		t.Run(filepath.Base(path), func(t *testing.T) {
			t.Parallel()
			fx := parseFixture(t, path)
			if p := fx.opt("pending", ""); p != "" {
				t.Skipf("pending: %s", p)
			}
			if fx.opt("stage", "sem") == "driver" {
				t.Skip("checked with a resolved target by internal/toolchain/driver's TestSemDriverFixtures")
			}
			var res *sem.Result
			srcName := "main/main.kg"
			if fx.opt("mir", "") == "planned" {
				res, srcName = checkPlanned(t, fx), "main.kg"
			} else {
				res = sem.CheckWith(buildModule(t, fx), schema.GenerateDerive)
			}
			bodies := map[string]string{}
			for _, s := range fx.sections {
				name := s.name
				if name == "src" {
					name = srcName
				}
				bodies[name] = s.body
			}
			var got strings.Builder
			for i := 1; i < len(res.Files); i++ {
				tree := res.Files[i].Tree
				got.WriteString(diag.RenderAll(res, res.Diagnostics(tree)))
				if body, ok := bodies[tree.File.Name]; ok {
					checkAnnotations(t, res, tree, body)
				} else if len(res.Diagnostics(tree)) > 0 {
					t.Errorf("shared stub %s reported diagnostics", tree.File.Name)
				}
			}
			if os.Getenv("UPDATE") == "1" {
				out := fx.raw[:fx.expectOff] + got.String()
				if err := os.WriteFile(path, []byte(out), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			if got.String() != fx.expect {
				t.Errorf("expect mismatch (UPDATE=1 to accept)\n--- got ---\n%s--- want ---\n%s", got.String(), fx.expect)
			}
		})
	}
}

// TestStdStubs checks the shared stubs alone: they must be clean.
func TestStdStubs(t *testing.T) {
	mod := &sem.Module{Packages: testkit.LoadStd(t, "../../../std")}
	res := sem.CheckWith(mod, schema.GenerateDerive)
	if out := res.Render(); out != "" {
		t.Errorf("std stubs are not clean:\n%s", out)
	}
}
