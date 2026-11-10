package vm_test

import (
	"go/ast"
	"go/parser"
	gotoken "go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/backend/llgen"
	"kigumi/internal/backend/llgen/runtimegen"
	"kigumi/internal/backend/vm"
	"kigumi/internal/compiler/mir"
	mirbuild "kigumi/internal/compiler/mir/build"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

// The runtime op manifest lives with the runtime generators: it joins the
// registry, the hand-written C dispatch and the VM's op table.
const manifestPath = "../llgen/runtimegen/runtime_ops_manifest.txt"

// vmGapReasons are the ops the VM deliberately does not run: `kigumi build`
// owns foreign memory and the dynamic loader. Every other live op must have a
// VM implementation; a gap without a reason fails the manifest.
var vmGapReasons = map[string]string{
	"CSTRING_NEW":        "std/ffi CString owns foreign memory, which the VM leaves to kigumi build",
	"CSTRING_PTR":        "std/ffi CString owns foreign memory, which the VM leaves to kigumi build",
	"CSTRING_DROP":       "std/ffi CString owns foreign memory, which the VM leaves to kigumi build",
	"FFI_BYTES_FROM":     "raw pointer to Bytes reads foreign memory, which the VM leaves to kigumi build",
	"FFI_BYTES_PTR":      "raw pointer to Bytes reads foreign memory, which the VM leaves to kigumi build",
	"FFI_STRING":         "a C string is foreign memory, which the VM leaves to kigumi build",
	"FFI_STRING_CHECKED": "a C string is foreign memory, which the VM leaves to kigumi build",
	"FFI_STRING_LOSSY":   "a C string is foreign memory, which the VM leaves to kigumi build",
	"SECURE_ERASE_RAW":   "raw secure erase touches foreign memory, which the VM leaves to kigumi build",
	"HOST_DL":            "the dynamic loader is only available through kigumi build",
	"DL_SYMBOL_CALL":     "calling a dynamically loaded symbol needs the loader of kigumi build",
}

// vmFamilySymbols name the VM function answering an op the registry keys by
// suffix or prefix alone, so no key literal names it.
var vmFamilySymbols = map[string]string{
	"EQUALS":       "prelude",
	"COMPARE_TO":   "prelude",
	"HASH":         "prelude",
	"BUILD_ONLY":   "registerBuild",
	"WRAPPING_ADD": "std",
	"WRAPPING_SUB": "std",
	"WRAPPING_MUL": "std",
	"WRAPPING_SHL": "std",
	"WRAPPING_SHR": "std",
	"ROTATE_LEFT":  "std",
	"ROTATE_RIGHT": "std",
}

// vmDispatcherFamilies are the key dispatchers of the VM in the order std tries
// them, so the manifest names the function that answers a key before the
// incidental tables (copy plans, result rules, argument roles) that mention it.
var vmDispatcherFamilies = []string{"collection", "tryFamily", "tryText", "prelude", "alloc", "memStd", "mathFn", "bytesStd", "byteBuffer", "intFn", "std"}

// TestRuntimeOpsImplemented holds every live registry op to an implementation
// on both backends and regenerates the manifest.
//
// C: the hand-written dispatch table must carry one body per live op, unguarded
// when the registry gives the op no capability and guarded by that capability
// (or one it needs) otherwise. That is runtimegen.OpBodies.
// VM: every bodiless std function resolving to the op must be answered by a
// host accelerator or a primitive; an op that only `kigumi build` can run must
// say why in vmGapReasons. The C dispatch and the VM are checked against each
// other through the registry, so a new op, key or body cannot land without both.
// std: every op must keep a key, suffix or prefix, so a std function resolves to it.
func TestRuntimeOpsImplemented(t *testing.T) {
	ops := llgen.RuntimeOps(opdef.Registry)
	res := checkSource(t, "print \"x\"\n")
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	impls := vmOpImplementations(t, ops, res, mirbuild.Program(res))
	got, err := runtimegen.RuntimeManifest(ops, impls)
	if err != nil {
		t.Fatalf("runtime op manifest: %v", err)
	}
	checkManifestFresh(t, got)
}

// vmOpImplementations derives the VM column from the VM itself: for each live
// op it collects how the VM runs every bodiless std function that resolves to
// it, then names the function (or table) answering the op's keys in the source.
func vmOpImplementations(t *testing.T, ops []runtimegen.Op, res *sem.Result, prog *mir.Program) map[string]runtimegen.VMImpl {
	t.Helper()
	kinds := map[string]map[vm.ImplementationKind][]string{}
	keys := map[string]bool{}
	for id := range res.Entities {
		fn := sem.EntityID(id)
		e := res.Entity(fn)
		if e.Kind != sem.EntFn || e.Flags&sem.EfStd == 0 || e.File == 0 || e.Name == "branch" {
			continue
		}
		info := res.Fn(fn)
		if info.Body != 0 || info.Abi != "" || res.Entity(e.Parent).Kind == sem.EntInterface {
			continue
		}
		key := opdef.StdKey(res, fn, true)
		keys[key] = true
		opID, ok := opdef.StdOpFor(key)
		if !ok {
			continue
		}
		name := opdef.StdOpNames[opID]
		if kinds[name] == nil {
			kinds[name] = map[vm.ImplementationKind][]string{}
		}
		kind := vm.Implementation(prog, fn)
		kinds[name][kind] = append(kinds[name][kind], key)
	}
	decls := vmDecls(t)
	impls := map[string]runtimegen.VMImpl{}
	for _, op := range ops {
		if op.Retired {
			continue
		}
		byKind := kinds[op.Name]
		if len(byKind) == 0 {
			t.Errorf("op %s: no bodiless std function resolves to it, so the VM column cannot be derived", op.Name)
			continue
		}
		if len(byKind) > 1 {
			t.Errorf("op %s: its std functions disagree on the VM implementation: %s", op.Name, describeKinds(byKind))
			continue
		}
		for kind, keys := range byKind {
			sort.Strings(keys)
			switch kind {
			case 0:
				reason := vmGapReasons[op.Name]
				if reason == "" {
					t.Errorf("op %s: no VM implementation for %s; implement it or declare why in vmGapReasons", op.Name, strings.Join(keys, ", "))
					continue
				}
				impls[op.Name] = runtimegen.VMImpl{Kind: runtimegen.VMGap, Reason: reason}
			case vm.ImplementedHost, vm.ImplementedPrimitive:
				if reason := vmGapReasons[op.Name]; reason != "" {
					t.Errorf("op %s: declared a VM gap (%q) but %s is implemented", op.Name, reason, strings.Join(keys, ", "))
					continue
				}
				implKind := runtimegen.VMStd
				if kind == vm.ImplementedHost {
					implKind = runtimegen.VMHost
				}
				symbol, ok := vmSymbol(op, implKind, decls)
				if !ok {
					t.Errorf("op %s: no VM source names %s; declare its family symbol in vmFamilySymbols", op.Name, strings.Join(op.Keys, ", "))
					continue
				}
				impls[op.Name] = runtimegen.VMImpl{Kind: implKind, Symbol: symbol}
			}
		}
	}
	live := map[string]bool{}
	for _, op := range ops {
		if op.Retired {
			continue
		}
		live[op.Name] = true
		for _, key := range op.Keys {
			if !keys[key] {
				t.Errorf("op %s: key %s names no bodiless std function", op.Name, key)
			}
		}
	}
	for name := range kinds {
		if !live[name] {
			t.Errorf("a bodiless std function resolves to %s, which is not a live registry op", name)
		}
	}
	for name := range vmGapReasons {
		if !live[name] {
			t.Errorf("vmGapReasons names %s, which is not a live registry op", name)
		}
	}
	return impls
}

func describeKinds(byKind map[vm.ImplementationKind][]string) string {
	names := map[vm.ImplementationKind]string{0: "no implementation", vm.ImplementedHost: "host", vm.ImplementedPrimitive: "primitive"}
	var kinds []int
	for kind := range byKind {
		kinds = append(kinds, int(kind))
	}
	sort.Ints(kinds)
	var parts []string
	for _, k := range kinds {
		keys := byKind[vm.ImplementationKind(k)]
		sort.Strings(keys)
		parts = append(parts, names[vm.ImplementationKind(k)]+" ("+strings.Join(keys, ", ")+")")
	}
	return strings.Join(parts, ", ")
}

// vmSymbol names the VM declarations answering an op: the function registering
// a host accelerator, or the key dispatcher of a primitive, one per key since
// two keys of one op may reach different dispatchers. An op the registry keys
// by suffix or prefix alone takes the family it declares.
func vmSymbol(op runtimegen.Op, kind runtimegen.VMKind, decls []vmDecl) (string, bool) {
	if len(op.Keys) == 0 {
		symbol, ok := vmFamilySymbols[op.Name]
		return symbol, ok
	}
	var symbols []string
	for _, key := range op.Keys {
		var cands []string
		for _, d := range decls {
			if strings.Contains(d.text, strconv.Quote(key)) {
				cands = append(cands, d.name)
			}
		}
		sort.Strings(cands)
		symbol := ""
		if kind == runtimegen.VMHost {
			for _, c := range cands {
				if strings.HasPrefix(c, "register") {
					symbol = c
					break
				}
			}
			if symbol == "" {
				for _, c := range cands {
					if c == "mathPrims" {
						symbol = c
						break
					}
				}
			}
		}
		if symbol == "" {
			for _, family := range vmDispatcherFamilies {
				if slices.Contains(cands, family) {
					symbol = family
					break
				}
			}
		}
		if symbol == "" && len(cands) > 0 {
			symbol = cands[0]
		}
		if symbol == "" {
			return "", false
		}
		symbols = append(symbols, symbol)
	}
	var joined []string
	for _, s := range symbols {
		if len(joined) == 0 || joined[len(joined)-1] != s {
			joined = append(joined, s)
		}
	}
	return strings.Join(joined, "+"), true
}

// vmDecl is one top-level VM declaration and the source it spans.
type vmDecl struct{ name, text string }

func vmDecls(t *testing.T) []vmDecl {
	t.Helper()
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var out []vmDecl
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		fset := gotoken.NewFileSet()
		file, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatal(err)
		}
		text := func(n ast.Node) string {
			return string(src[fset.Position(n.Pos()).Offset:fset.Position(n.End()).Offset])
		}
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				out = append(out, vmDecl{d.Name.Name, text(d)})
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.ValueSpec:
						for _, name := range s.Names {
							out = append(out, vmDecl{name.Name, text(d)})
						}
					case *ast.TypeSpec:
						out = append(out, vmDecl{s.Name.Name, text(d)})
					}
				}
			}
		}
	}
	return out
}

// checkManifestFresh compares the rendered manifest with the checked-in one;
// UPDATE=1 rewrites it.
func checkManifestFresh(t *testing.T, got string) {
	t.Helper()
	if os.Getenv("UPDATE") == "1" {
		if err := os.WriteFile(manifestPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("%v; run UPDATE=1 go test ./internal/backend/vm -run TestRuntimeOpsImplemented", err)
	}
	if string(want) == got {
		return
	}
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(string(want), "\n")
	var diffs []string
	for i := 0; i < len(gotLines) || i < len(wantLines); i++ {
		switch {
		case i >= len(gotLines):
			diffs = append(diffs, strconv.Itoa(i+1)+": manifest has no line, want "+wantLines[i])
		case i >= len(wantLines):
			diffs = append(diffs, strconv.Itoa(i+1)+": manifest has "+gotLines[i]+", want no line")
		case gotLines[i] != wantLines[i]:
			diffs = append(diffs, strconv.Itoa(i+1)+":\n  have "+gotLines[i]+"\n  want "+wantLines[i])
		}
	}
	if len(diffs) > 10 {
		diffs = append(diffs[:10], "...")
	}
	t.Fatalf("%s is stale; run UPDATE=1 go test ./internal/backend/vm -run TestRuntimeOpsImplemented\n%s", manifestPath, strings.Join(diffs, "\n"))
}
