package vm_test

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
	"kigumi/internal/compiler/hir"
	mirbuild "kigumi/internal/compiler/mir/build"
	mirplan "kigumi/internal/compiler/mir/plan"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	devfixture "kigumi/internal/devkit/fixture"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
)

type fixture struct {
	src, stdout string
	exit        int
}

// runFixturePath locates a run fixture by name, independent of its group.
func runFixturePath(t *testing.T, name string) string {
	t.Helper()
	path, err := devfixture.Find("../../..", "run/"+name)
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func readFixture(t *testing.T, path string) fixture {
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	var fx fixture
	sections := strings.Split(text, "\n-- ")
	for i, s := range sections {
		if i == 0 {
			s = strings.TrimPrefix(s, "-- ")
		}
		name, body, _ := strings.Cut(s, " --\n")
		if i < len(sections)-1 {
			body += "\n"
		}
		switch name {
		case "src":
			fx.src = body
		case "stdout":
			fx.stdout = body
		case "exit":
			fx.exit, _ = strconv.Atoi(strings.TrimSpace(body))
		}
	}
	return fx
}

func checkSource(t *testing.T, src string) *sem.Result {
	pkgs := map[string]*sem.Package{}
	for _, p := range semfixture.LoadStd(t, "../../../std") {
		pkgs[p.Path] = p
	}
	tree := syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	pkgs["main"] = &sem.Package{Path: "main", Files: []*syntax.Tree{tree}, Entry: tree}
	var paths []string
	for p := range pkgs {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	mod := &sem.Module{}
	for _, p := range paths {
		mod.Packages = append(mod.Packages, pkgs[p])
	}
	return sem.CheckWith(mod, schema.GenerateDerive)
}

// The scalar fixture runs through HIR, MIR and the VM and prints what the
// AOT corpus prints for the same source; the HIR dump pins what the
// checker decided.
func TestScalarSlice(t *testing.T) {
	fx := readFixture(t, runFixturePath(t, "hir_scalar"))
	res := checkSource(t, fx.src)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	var dump strings.Builder
	for id := 1; id < len(res.Entities); id++ {
		e := res.Entity(sem.EntityID(id))
		if e.File == 0 || res.Packages[e.Pkg].Path != "main" || (e.Kind != sem.EntFn && e.Kind != sem.EntImplicitMain) {
			continue
		}
		if f := hir.Lower(res, sem.EntityID(id)); f != nil {
			dump.WriteString(hir.Dump(res, f))
		}
	}
	prog := mirbuild.Program(res)
	if prog.HasErrors() {
		t.Fatal("ownership errors")
	}
	if err := mirplan.Apply(prog, mirplan.Transform{Name: "plan-rc", Run: mirplan.RC}); err != nil {
		t.Fatalf("plan-rc: %v", err)
	}
	var out, errOut bytes.Buffer
	if code, err := vm.Run(prog, vm.Options{}, &out, &errOut); err != nil || code != 0 {
		t.Fatalf("vm: %v (exit %d)\n%s", err, code, errOut.String())
	}
	if out.String() != fx.stdout {
		t.Errorf("stdout mismatch\n--- got ---\n%s--- want ---\n%s", out.String(), fx.stdout)
	}
	golden := filepath.Join("..", "..", "..", "testdata", "golden", "hir", "scalar.hir")
	if os.Getenv("UPDATE") != "" {
		os.WriteFile(golden, []byte(dump.String()), 0o644)
	}
	wantDump, err := os.ReadFile(golden)
	if err != nil || string(wantDump) != dump.String() {
		t.Errorf("HIR dump differs from %s (UPDATE=1 to accept)\n%s", golden, dump.String())
	}
}
