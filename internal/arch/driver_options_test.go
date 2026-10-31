package arch_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// ambientGlobals are what the driver may touch only in the files that resolve CompilerOptions.
var ambientGlobals = map[string]bool{
	"os.UserCacheDir": true, "os.UserHomeDir": true, "os.Getwd": true, "os.TempDir": true,
	"runtime.GOOS": true, "runtime.GOARCH": true,
}

func parseDriver(t *testing.T) map[string]*ast.File {
	t.Helper()
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	paths, _ := filepath.Glob("../toolchain/driver/*.go")
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files[filepath.Base(p)] = f
	}
	return files
}

func TestDriverReadsNoAmbientPlatform(t *testing.T) {
	for name, f := range parseDriver(t) {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok && ambientGlobals[id.Name+"."+x.Sel.Name] {
					t.Errorf("internal/toolchain/driver/%s: %s.%s を読んでいる。internal/cli が CompilerOptions に入れて渡せ", name, id.Name, x.Sel.Name)
				}
			case *ast.CallExpr:
				name := ""
				if id, ok := x.Fun.(*ast.Ident); ok {
					name = id.Name
				}
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
					if owner, ok := sel.X.(*ast.Ident); ok && owner.Name == "buildconfig" {
						name = sel.Sel.Name
					}
				}
				if name == "HostTarget" {
					t.Errorf("internal/toolchain/driver/%s: HostTarget() を呼んでいる。CompilerOptions.HostOrAmbient を使え", name)
				}
			}
			return true
		})
	}
}

func TestDriverEntryPointsTakeCompilerOptions(t *testing.T) {
	files := parseDriver(t)
	want := map[string]bool{
		"CacheDir": false, "HeaderCacheDir": false, "DeriveCacheDir": false,
		"DepDir": false, "ResolveGraph": false, "SelectVersions": false, "IsHostTarget": false, "ResolveLinkIdentity": false,
	}
	hasField := false
	for _, f := range files {
		for _, d := range f.Decls {
			switch x := d.(type) {
			case *ast.FuncDecl:
				if _, ok := want[x.Name.Name]; ok && x.Recv == nil {
					want[x.Name.Name] = firstParamIs(x, "CompilerOptions")
				}
			case *ast.GenDecl:
				for _, s := range x.Specs {
					if ts, ok := s.(*ast.TypeSpec); ok && ts.Name.Name == "LoadOptions" {
						hasField = structHasField(ts, "Compiler", "CompilerOptions")
					}
				}
			}
		}
	}
	for name, ok := range want {
		if !ok {
			t.Errorf("driver.%s は最初の引数に CompilerOptions を取る必要がある", name)
		}
	}
	targetParser := false
	for _, f := range parseDir(t, "../toolchain/config", false) {
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "ParseTarget" {
				targetParser = firstParamIs(fn, "CompilerOptions")
			}
		}
	}
	if !targetParser {
		t.Error("config.ParseTarget must take CompilerOptions first")
	}
	if !hasField {
		t.Error("driver.LoadOptions に Compiler CompilerOptions がない")
	}
}

func firstParamIs(fn *ast.FuncDecl, typ string) bool {
	params := fn.Type.Params.List
	return len(params) > 0 && optionType(params[0].Type, typ)
}

func optionType(expr ast.Expr, typ string) bool {
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name == typ
	}
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	owner, ok := sel.X.(*ast.Ident)
	return ok && owner.Name == "buildconfig" && sel.Sel.Name == typ
}

func structHasField(ts *ast.TypeSpec, name, typ string) bool {
	st, ok := ts.Type.(*ast.StructType)
	if !ok {
		return false
	}
	for _, f := range st.Fields.List {
		for _, n := range f.Names {
			if n.Name == name && optionType(f.Type, typ) {
				return true
			}
		}
	}
	return false
}
