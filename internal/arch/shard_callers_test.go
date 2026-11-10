package arch_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

const (
	devkitPath = "kigumi/internal/devkit/"
	fixturePkg = devkitPath + "fixture"
)

// declGraph links each declaration (`<package> <name>`, methods as `<package> .<name>`) to the
// declarations its body names, across the scanned packages.
type declGraph struct {
	refs    map[string][]string
	methods map[string][]string
	tests   []string
}

// shardCallers lists the Test functions of the package in dir (import path pkg) that reach
// fixture.Shards through that directory's files or any devkit package.
func shardCallers(t *testing.T, root, dir, pkg string) []string {
	t.Helper()
	g := declGraph{refs: map[string][]string{}, methods: map[string][]string{}}
	devkit := filepath.Join(root, "internal", "devkit")
	err := filepath.WalkDir(devkit, func(p string, d os.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(devkit, p)
		g.addDir(t, p, path.Join(devkitPath, filepath.ToSlash(rel)), false)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	g.addDir(t, dir, pkg, true)
	reach := g.reachingShard()
	var out []string
	for _, test := range g.tests {
		if reach[test] {
			_, name, _ := strings.Cut(test, " ")
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// addDir scans a package directory; withTests also scans its test files and records their Test functions.
func (g *declGraph) addDir(t *testing.T, dir, pkg string, withTests bool) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	for _, file := range files {
		isTest := strings.HasSuffix(file, "_test.go")
		if isTest && !withTests {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		id := pkg
		if strings.HasSuffix(f.Name.Name, "_test") {
			id = pkg + "_test"
		}
		g.addFile(f, id, pkg, isTest)
	}
}

func (g *declGraph) addFile(f *ast.File, id, pkg string, isTest bool) {
	imports := map[string]string{}
	for _, imp := range f.Imports {
		p, _ := strconv.Unquote(imp.Path.Value)
		name := path.Base(p)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		imports[name] = ""
		if p == pkg || strings.HasPrefix(p, devkitPath) {
			imports[name] = p
		}
	}
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			key := id + " " + d.Name.Name
			if d.Recv != nil {
				key = id + " ." + d.Name.Name
				g.methods[d.Name.Name] = append(g.methods[d.Name.Name], key)
			} else if isTest && strings.HasPrefix(d.Name.Name, "Test") {
				g.tests = append(g.tests, key)
			}
			g.scan(key, id, imports, d.Body)
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					for _, name := range vs.Names {
						for _, v := range vs.Values {
							g.scan(id+" "+name.Name, id, imports, v)
						}
					}
				}
			}
		}
	}
}

// scan records every declaration the node names. A selector on a value may be a method of any
// scanned package, so it links to every method of that name.
func (g *declGraph) scan(key, id string, imports map[string]string, n ast.Node) {
	if n == nil {
		return
	}
	ast.Inspect(n, func(n ast.Node) bool {
		switch e := n.(type) {
		case *ast.SelectorExpr:
			if x, ok := e.X.(*ast.Ident); ok {
				if p, isImport := imports[x.Name]; isImport {
					if p != "" {
						g.refs[key] = append(g.refs[key], p+" "+e.Sel.Name)
					}
					return false
				}
			}
			g.refs[key] = append(g.refs[key], "method "+e.Sel.Name)
		case *ast.Ident:
			g.refs[key] = append(g.refs[key], id+" "+e.Name)
		}
		return true
	})
}

// reachingShard grows the set of declarations that reach fixture.Shards backwards along the
// references until it is stable, so a cycle between helpers cannot hide a caller.
func (g *declGraph) reachingShard() map[string]bool {
	reach := map[string]bool{fixturePkg + " Shards": true}
	reached := func(ref string) bool {
		if name, ok := strings.CutPrefix(ref, "method "); ok {
			return slices.ContainsFunc(g.methods[name], func(m string) bool { return reach[m] })
		}
		return reach[ref]
	}
	for changed := true; changed; {
		changed = false
		for key, refs := range g.refs {
			if !reach[key] && slices.ContainsFunc(refs, reached) {
				reach[key], changed = true, true
			}
		}
	}
	return reach
}
