package arch_test

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// quadrupleFields are the four target facts that must always be minted
// together; policy fields (Allocator, Profile, BlockingExecutor, stack) are
// deliberately absent because callers set those independently.
var quadrupleFields = map[string]bool{"Triple": true, "OS": true, "Arch": true, "Sys": true}

// TestTargetQuadrupleMintedOnlyByConfig guards the one target constructor:
// no package outside internal/toolchain/config may assemble a config.Target's
// Triple/OS/Arch/Sys from a composite literal, because those four facts must
// come from TargetFromTriple (or the explicit HostTarget) or they can drift
// apart. Test files are exempt: table-driven tests build targets by literal
// on purpose.
func TestTargetQuadrupleMintedOnlyByConfig(t *testing.T) {
	t.Parallel()
	fset := token.NewFileSet()
	configDir, err := filepath.Abs("../toolchain/config")
	if err != nil {
		t.Fatal(err)
	}
	files := 0
	err = filepath.WalkDir("..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path == ".." {
				return nil
			}
			abs, err := filepath.Abs(path)
			if err != nil {
				return err
			}
			if abs == configDir || strings.HasPrefix(d.Name(), ".") || d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		files++
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Contains(src, []byte("config.Target{")) {
			return nil
		}
		f, err := parser.ParseFile(fset, path, src, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok || !isConfigTargetLit(lit.Type) {
				return true
			}
			if len(lit.Elts) == 0 {
				return true
			}
			for _, e := range lit.Elts {
				kv, ok := e.(*ast.KeyValueExpr)
				if !ok {
					t.Errorf("%s: positional config.Target literal bypasses TargetFromTriple", fset.Position(lit.Pos()))
					break
				}
				if key, ok := kv.Key.(*ast.Ident); ok && quadrupleFields[key.Name] {
					t.Errorf("%s: config.Target literal sets %s; mint the target with TargetFromTriple or HostTarget", fset.Position(kv.Pos()), key.Name)
				}
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if files == 0 {
		t.Fatal("the guard scanned no Go files; the walk is broken")
	}
}

func isConfigTargetLit(expr ast.Expr) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Target" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && (id.Name == "config" || id.Name == "buildconfig")
}
