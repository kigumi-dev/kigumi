package arch_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoExecWithLiteralProgram(t *testing.T) {
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
		if !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") || strings.HasPrefix(rel, "internal/cli/") ||
			!(strings.HasPrefix(rel, "internal/") || strings.HasPrefix(rel, "cmd/")) {
			return nil
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			return perr
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
			if id, ok := sel.X.(*ast.Ident); !ok || id.Name != "exec" {
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
			if len(call.Args) > arg {
				if lit, ok := call.Args[arg].(*ast.BasicLit); ok && lit.Kind == token.STRING {
					t.Errorf("%s:%d: exec.%s に文字列リテラルのプログラム名を渡している。Lookup が返したパスを使え", rel, fset.Position(call.Pos()).Line, sel.Sel.Name)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
