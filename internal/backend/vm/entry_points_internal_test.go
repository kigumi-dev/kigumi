package vm

import (
	"go/ast"
	"go/parser"
	gotoken "go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	opdef "kigumi/internal/compiler/mir/ops"
)

// TestBuiltinTableCoversBuiltins gives every builtin the VM runs a label rule, and names no other.
func TestBuiltinTableCoversBuiltins(t *testing.T) {
	for _, name := range opdef.BuiltinNames() {
		if spec, _ := opdef.BuiltinFor(name); spec.VM {
			if _, ok := builtinTable[name]; !ok {
				t.Errorf("builtin %s has no rule in builtinTable", name)
			}
		}
	}
	for name := range builtinTable {
		if spec, ok := opdef.BuiltinFor(name); !ok || !spec.VM {
			t.Errorf("builtinTable names %s, which the VM does not run", name)
		}
	}
}

// allowedEntry is the only Machine method allowed to reach each kind of entry that answers a primitive or a builtin.
var allowedEntry = map[string][]string{
	"std":     {"Machine.callPrim"},
	"handler": {"Machine.std"},
	"host":    {"Machine.callPrim", "Machine.runDropHook"},
	"builtin": {"Machine.callBuiltin"},
}

// vmDecl is one top-level declaration of the package, named by its receiver type and name.
type vmDecl struct {
	path, owner string
	node        ast.Node
	fn          *ast.FuncDecl
}

// TestPrimitivesEnterThroughOneEntry keeps every primitive behind callPrim (a drop hook takes only the resource) and
// every builtin behind callBuiltin, counting method values, any receiver, host-table ranges and package-level literals.
func TestPrimitivesEnterThroughOneEntry(t *testing.T) {
	decls := vmDecls(t)
	var std *ast.FuncDecl
	for _, d := range decls {
		if d.owner == "Machine.std" {
			std = d.fn
		}
	}
	handlers := stdHandlers(std)
	if len(handlers) < 5 {
		t.Fatalf("found %d handlers in std; the scan no longer sees the dispatch", len(handlers))
	}
	guarded := map[string]string{"std": "std", "builtin": "builtin"}
	for h := range handlers {
		guarded[h] = "handler"
	}
	seen := map[string]int{}
	for _, d := range decls {
		hosts := hostBindings(d.node)
		ast.Inspect(d.node, func(n ast.Node) bool {
			kind := ""
			switch x := n.(type) {
			case *ast.SelectorExpr:
				if _, isIdent := x.X.(*ast.Ident); isIdent {
					kind = guarded[x.Sel.Name]
				}
			case *ast.CallExpr:
				kind = hostCallKind(x, hosts)
			}
			if kind == "" {
				return true
			}
			seen[kind]++
			if !slices.Contains(allowedEntry[kind], d.owner) {
				t.Errorf("%s: %s reaches a %s entry outside %v", d.path, d.owner, kind, allowedEntry[kind])
			}
			return true
		})
	}
	for kind := range allowedEntry {
		if seen[kind] == 0 {
			t.Errorf("no %s entry found; the scan no longer sees it", kind)
		}
	}
}

// vmDecls parses the package's non-test files into their top-level declarations.
func vmDecls(t *testing.T) []vmDecl {
	t.Helper()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var out []vmDecl
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(gotoken.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range file.Decls {
			d := vmDecl{path: path, owner: "package level", node: decl}
			if fn, ok := decl.(*ast.FuncDecl); ok {
				d.fn, d.owner = fn, fn.Name.Name
				if fn.Recv != nil && len(fn.Recv.List) > 0 {
					d.owner = recvName(fn.Recv.List[0].Type) + "." + fn.Name.Name
				}
			}
			out = append(out, d)
		}
	}
	return out
}

func recvName(e ast.Expr) string {
	if star, ok := e.(*ast.StarExpr); ok {
		e = star.X
	}
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return "?"
}

// hostCallKind reports a call of a host function: a bound one, a plan's, or one indexed out of the host table.
func hostCallKind(call *ast.CallExpr, hosts map[string]bool) string {
	switch f := call.Fun.(type) {
	case *ast.SelectorExpr:
		if f.Sel.Name == "host" {
			return "host"
		}
	case *ast.Ident:
		if hosts[f.Name] {
			return "host"
		}
	case *ast.IndexExpr:
		if hostTable(f.X) {
			return "host"
		}
	}
	return ""
}

func hostTable(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	return ok && (sel.Sel.Name == "host" || sel.Sel.Name == "hostByOp")
}

// stdHandlers are the methods std calls on its own receiver with the key or the arguments.
func stdHandlers(std *ast.FuncDecl) map[string]bool {
	out := map[string]bool{}
	if std == nil || std.Recv == nil || len(std.Recv.List[0].Names) == 0 {
		return out
	}
	self := std.Recv.List[0].Names[0].Name
	ast.Inspect(std.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if x, isIdent := sel.X.(*ast.Ident); !isIdent || x.Name != self {
			return true
		}
		for _, arg := range call.Args {
			if id, ok := arg.(*ast.Ident); ok && (id.Name == "key" || id.Name == "a") {
				out[sel.Sel.Name] = true
			}
		}
		return true
	})
	return out
}

// hostBindings names the identifiers in decl that hold a host function: parameters of type hostFn, values bound from
// hostFor, hostForEnt, hostDrop or the host table, and range variables over the host table.
func hostBindings(decl ast.Node) map[string]bool {
	out := map[string]bool{}
	ast.Inspect(decl, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncType:
			for _, field := range x.Params.List {
				if id, ok := field.Type.(*ast.Ident); ok && id.Name == "hostFn" {
					for _, name := range field.Names {
						out[name.Name] = true
					}
				}
			}
		case *ast.RangeStmt:
			if id, ok := x.Value.(*ast.Ident); ok && hostTable(x.X) {
				out[id.Name] = true
			}
		case *ast.AssignStmt:
			if len(x.Rhs) != 1 || len(x.Lhs) == 0 {
				return true
			}
			id, ok := x.Lhs[0].(*ast.Ident)
			if !ok {
				return true
			}
			switch rhs := x.Rhs[0].(type) {
			case *ast.CallExpr:
				if sel, ok := rhs.Fun.(*ast.SelectorExpr); ok {
					switch sel.Sel.Name {
					case "hostFor", "hostForEnt", "hostDrop":
						out[id.Name] = true
					}
				}
			case *ast.IndexExpr:
				if hostTable(rhs.X) {
					out[id.Name] = true
				}
			}
		}
		return true
	})
	return out
}
