package vm

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strconv"
	"testing"

	"kigumi/internal/mir"
)

func TestBuiltinCasesMatchTheRegistry(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "builtins.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var cases []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "builtin" {
			continue
		}
		for _, stmt := range fn.Body.List {
			sw, ok := stmt.(*ast.SwitchStmt)
			if !ok {
				continue
			}
			for _, c := range sw.Body.List {
				for _, e := range c.(*ast.CaseClause).List {
					if lit, ok := e.(*ast.BasicLit); ok {
						name, _ := strconv.Unquote(lit.Value)
						cases = append(cases, name)
					}
				}
			}
		}
	}
	var want []string
	for _, name := range mir.BuiltinNames() {
		if spec, _ := mir.BuiltinFor(name); spec.VM {
			want = append(want, name)
		}
	}
	slices.Sort(cases)
	slices.Sort(want)
	if !slices.Equal(cases, want) {
		t.Fatalf("Machine.builtin runs %v, the registry says the VM runs %v", cases, want)
	}
}
