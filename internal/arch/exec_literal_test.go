package arch_test

import (
	"errors"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

type noImports struct{}

func (noImports) Import(path string) (*types.Package, error) { return nil, errors.New("not loaded") }

// TestNoExecWithLiteralProgram forbids a constant program name in exec.Command(Context), whether written as a literal, a named const or a concatenation. There is no exemption: internal/cli resolves tools through session.CompilerOptions like every other package.
// Constants are folded per package with go/types and imports left unresolved, so a constant imported from another package is not seen.
func TestNoExecWithLiteralProgram(t *testing.T) {
	byDir := map[string][]string{}
	err := filepath.WalkDir("../..", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == "node_modules" {
				return fs.SkipDir
			}
			return nil
		}
		rel := filepath.ToSlash(strings.TrimPrefix(p, "../../"))
		if strings.HasSuffix(rel, ".go") && !strings.HasSuffix(rel, "_test.go") && (strings.HasPrefix(rel, "internal/") || strings.HasPrefix(rel, "cmd/")) {
			byDir[filepath.Dir(p)] = append(byDir[filepath.Dir(p)], p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, paths := range byDir {
		checkExecPackage(t, paths)
	}
}

func checkExecPackage(t *testing.T, paths []string) {
	fset := token.NewFileSet()
	var files []*ast.File
	for _, p := range paths {
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}
	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}}
	conf := types.Config{Importer: noImports{}, Error: func(error) {}}
	conf.Check("p", fset, files, info)
	for _, f := range files {
		execName := ""
		for _, im := range f.Imports {
			if im.Path.Value != `"os/exec"` {
				continue
			}
			execName = "exec"
			if im.Name != nil {
				execName = im.Name.Name
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); !ok || id.Name != execName {
				return true
			}
			arg := 0
			switch sel.Sel.Name {
			case "Command":
			case "CommandContext":
				arg = 1
			default:
				return true
			}
			if len(call.Args) <= arg {
				return true
			}
			tv := info.Types[call.Args[arg]]
			if lit, ok := call.Args[arg].(*ast.BasicLit); (ok && lit.Kind == token.STRING) || (tv.Value != nil && tv.Value.Kind() == constant.String) {
				t.Errorf("%s: exec.%s に定数の文字列をプログラム名として渡している。Lookup が返したパスを使え", fset.Position(call.Pos()), sel.Sel.Name)
			}
			return true
		})
	}
}
