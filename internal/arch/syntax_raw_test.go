package arch_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// rawTreeNames is the raw node-record API that the typed views replaced.
var rawTreeNames = []string{"Node", "Nodes", "Extra", "Lhs", "Rhs", "Slot", "Slots", "SlotNames", "PathToks", "Children", "EachChild"}

func parseDir(t *testing.T, dir string, withTests bool) map[string]*ast.File {
	t.Helper()
	files := map[string]*ast.File{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "testdata" || d.Name() == "node_modules" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || !withTests && strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		files[filepath.ToSlash(p)] = f
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

// onTree is true for package-level functions and methods of Tree, the types
// the raw accessors lived on.
func onTree(d *ast.FuncDecl) bool {
	if d.Recv == nil {
		return true
	}
	typ := d.Recv.List[0].Type
	if star, ok := typ.(*ast.StarExpr); ok {
		typ = star.X
	}
	id, ok := typ.(*ast.Ident)
	return ok && id.Name == "Tree"
}

func TestSyntaxExportsNoRawTreeAPI(t *testing.T) {
	for path, f := range parseDir(t, "../compiler/syntax", false) {
		if strings.Contains(path, "/syntax/testdata/") {
			continue
		}
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if slices.Contains(rawTreeNames, d.Name.Name) && d.Name.IsExported() && onTree(d) {
					t.Errorf("%s: exported %s exposes the raw node record; add a typed view accessor instead", path, d.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					if slices.Contains(rawTreeNames, ts.Name.Name) {
						t.Errorf("%s: exported type %s exposes the raw node record", path, ts.Name.Name)
					}
					st, ok := ts.Type.(*ast.StructType)
					if !ok || !ts.Name.IsExported() {
						continue
					}
					for _, fld := range st.Fields.List {
						for _, n := range fld.Names {
							if slices.Contains(rawTreeNames, n.Name) {
								t.Errorf("%s: exported field %s.%s exposes the raw node record", path, ts.Name.Name, n.Name)
							}
						}
					}
				}
			}
		}
	}
}

func TestNoRawTreeAccessOutsideSyntax(t *testing.T) {
	selectors := []string{"Nodes", "Extra", "Slot", "Slots", "SlotNames", "PathToks"}
	for path, f := range parseDir(t, "..", true) {
		if strings.HasPrefix(path, "../compiler/syntax/") || !importsSyntax(f) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "syntax" && sel.Sel.Name == "Node" {
				t.Errorf("%s: syntax.Node is gone; read the tree through syntax views", path)
			}
			if slices.Contains(selectors, sel.Sel.Name) {
				t.Errorf("%s: .%s reads the raw syntax tree; use the typed views in internal/compiler/syntax", path, sel.Sel.Name)
			}
			return true
		})
	}
}

func importsSyntax(f *ast.File) bool {
	for _, imp := range f.Imports {
		if imp.Path.Value == `"kigumi/internal/compiler/syntax"` {
			return true
		}
	}
	return false
}
