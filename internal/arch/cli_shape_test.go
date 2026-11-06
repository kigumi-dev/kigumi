package arch_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCLICommandShape keeps cobra in cmd.go and options.go of every cli/cmd package, with New (and run.Script) the only exports.
func TestCLICommandShape(t *testing.T) {
	dirs, err := filepath.Glob("../cli/cmd/*")
	if err != nil || len(dirs) == 0 {
		t.Fatalf("no command packages: %v", err)
	}
	for _, dir := range dirs {
		pkg := filepath.Base(dir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		funcs := map[string]string{}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, parser.SkipObjectResolution)
			if err != nil {
				t.Fatal(err)
			}
			checkCLIFile(t, pkg, name, f, funcs)
		}
		if funcs["New"] != "cmd.go" {
			t.Errorf("cli/cmd/%s: cmd.go に func New() *cobra.Command を置け", pkg)
		}
		if funcs["parseOptions"] != "options.go" {
			t.Errorf("cli/cmd/%s: options.go に parseOptions を置け", pkg)
		}
	}
}

func checkCLIFile(t *testing.T, pkg, name string, f *ast.File, funcs map[string]string) {
	t.Helper()
	if name != "cmd.go" && name != "options.go" {
		for _, imp := range f.Imports {
			if imp.Path.Value == `"github.com/spf13/cobra"` {
				t.Errorf("cli/cmd/%s/%s: cobra は cmd.go と options.go だけが import する。run 側は session.IO と options を受け取れ", pkg, name)
			}
		}
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv != nil {
				continue
			}
			funcs[d.Name.Name] = name
			if d.Name.IsExported() && d.Name.Name != "New" && !(pkg == "run" && d.Name.Name == "Script") {
				t.Errorf("cli/cmd/%s/%s: %s を export している。command package の export は New だけ", pkg, name, d.Name.Name)
			}
		case *ast.GenDecl:
			for _, s := range d.Specs {
				for _, id := range specNames(s) {
					if id.IsExported() {
						t.Errorf("cli/cmd/%s/%s: %s を export している。command package の export は New だけ", pkg, name, id.Name)
					}
				}
			}
		}
	}
}

func specNames(s ast.Spec) []*ast.Ident {
	switch s := s.(type) {
	case *ast.TypeSpec:
		return []*ast.Ident{s.Name}
	case *ast.ValueSpec:
		return s.Names
	}
	return nil
}

// TestCLITestOnlyInTests keeps cli/clitest, which sets KIGUMI_VERIFY in its init, out of the shipped binary.
func TestCLITestOnlyInTests(t *testing.T) {
	err := filepath.WalkDir("../..", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == "node_modules" || d.Name() == "docs-new" || d.Name() == ".work" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), p, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			if imp.Path.Value == `"kigumi/internal/cli/clitest"` {
				t.Errorf("%s: cli/clitest は test からだけ import する", p)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
