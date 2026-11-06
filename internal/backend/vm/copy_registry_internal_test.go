package vm

import (
	"go/ast"
	"go/parser"
	gotoken "go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	opdef "kigumi/internal/compiler/mir/ops"
)

func TestNativeCopyPrimitivesUsePlans(t *testing.T) {
	funcs, cases := copyPrimitiveBodies(t)
	keys := map[string][]string{}
	for _, op := range opdef.Registry.Ops {
		keys[op.Name] = op.Keys
	}
	check := func(key string) {
		t.Helper()
		plan, ok := tryPlans[key]
		if !ok || plan.copy == copyOk {
			t.Errorf("%s needs a panicking copy plan", key)
		}
		if !callsPlannedCopy(cases[key], funcs, map[string]bool{}) {
			t.Errorf("%s bypasses copiedResult", key)
		}
	}
	count := 0
	err := filepath.WalkDir("../llgen/runtimegen/runtime", func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".c") && !strings.HasSuffix(path, ".inc") {
			return err
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, block := range strings.Split(string(source), "case OP_")[1:] {
			name, body, ok := strings.Cut(block, ":")
			if !ok {
				continue
			}
			body, _, _ = strings.Cut(body, "#endif")
			if !strings.Contains(body, "rt_copy(") {
				continue
			}
			count++
			if len(keys[name]) == 0 {
				t.Errorf("%s: copying OP_%s has no registered key", path, name)
			}
			for _, key := range keys[name] {
				check(key)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("no native copying primitives found")
	}
	for _, key := range []string{"error.as", "fixed.fill"} {
		check(key)
	}
}

func copyPrimitiveBodies(t *testing.T) (map[string]*ast.FuncDecl, map[string]ast.Node) {
	t.Helper()
	funcs, cases := map[string]*ast.FuncDecl{}, map[string]ast.Node{}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(gotoken.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				funcs[fn.Name.Name] = fn
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.CaseClause:
				for _, expr := range n.List {
					if key, ok := copyPrimitiveKey(expr); ok {
						cases[key] = n
					}
				}
			case *ast.AssignStmt:
				if len(n.Lhs) != 1 || len(n.Rhs) != 1 {
					break
				}
				index, indexed := n.Lhs[0].(*ast.IndexExpr)
				fn, closure := n.Rhs[0].(*ast.FuncLit)
				if indexed && closure {
					if key, ok := copyPrimitiveKey(index.Index); ok {
						cases[key] = fn.Body
					}
				}
			}
			return true
		})
	}
	return funcs, cases
}

func copyPrimitiveKey(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != gotoken.STRING {
		return "", false
	}
	key, err := strconv.Unquote(lit.Value)
	return key, err == nil
}

func callsPlannedCopy(node ast.Node, funcs map[string]*ast.FuncDecl, seen map[string]bool) bool {
	if node == nil {
		return false
	}
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if _, closure := n.(*ast.FuncLit); closure {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok || found {
			return !found
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		recv, ok := sel.X.(*ast.Ident)
		if !ok || recv.Name != "m" {
			return true
		}
		name := sel.Sel.Name
		if name == "copiedResult" {
			found = true
		} else if fn := funcs[name]; (name == "tryFilterItems" || name == "fillFixed") && fn != nil && !seen[name] {
			seen[name] = true
			found = callsPlannedCopy(fn.Body, funcs, seen)
		}
		return !found
	})
	return found
}
