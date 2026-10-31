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
		if name == "compiler_options.go" || name == "compiler_tools.go" {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if id, ok := x.X.(*ast.Ident); ok && ambientGlobals[id.Name+"."+x.Sel.Name] {
					t.Errorf("internal/toolchain/driver/%s: %s.%s を読んでいる。internal/cli が CompilerOptions に入れて渡せ", name, id.Name, x.Sel.Name)
				}
			case *ast.CallExpr:
				if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "HostTarget" {
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
		"ParseTarget": false, "CacheDir": false, "HeaderCacheDir": false, "DeriveCacheDir": false,
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
	if !hasField {
		t.Error("driver.LoadOptions に Compiler CompilerOptions がない")
	}
}

func firstParamIs(fn *ast.FuncDecl, typ string) bool {
	params := fn.Type.Params.List
	if len(params) == 0 {
		return false
	}
	id, ok := params[0].Type.(*ast.Ident)
	return ok && id.Name == typ
}

func structHasField(ts *ast.TypeSpec, name, typ string) bool {
	st, ok := ts.Type.(*ast.StructType)
	if !ok {
		return false
	}
	for _, f := range st.Fields.List {
		id, isIdent := f.Type.(*ast.Ident)
		for _, n := range f.Names {
			if n.Name == name && isIdent && id.Name == typ {
				return true
			}
		}
	}
	return false
}
