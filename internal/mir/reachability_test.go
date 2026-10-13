package mir_test

import (
	"encoding/json"
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/schema"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
)

const reachSrc = `
fn used() -> i32 { 1 }
fn unused() -> i32 { 2 }

export(C) {
	pub fn exported() -> i32 { 3 }
}

let _ = used()
print "ok"
`

func checkedReach(t *testing.T) (*sem.Result, *mir.Program) {
	t.Helper()
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(reachSrc)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	return res, mir.Build(res)
}

func entNamed(t *testing.T, r *sem.Result, name string) sem.EntityID {
	t.Helper()
	for id := 1; id < len(r.Entities); id++ {
		if e := r.Entity(sem.EntityID(id)); e.Kind == sem.EntFn && e.Name == name {
			return sem.EntityID(id)
		}
	}
	t.Fatalf("no EntFn named %q", name)
	return 0
}

func TestReachableKeepsCallGraphAndRoots(t *testing.T) {
	res, prog := checkedReach(t)
	rs := mir.Reachable(prog)

	used, unused, exported := entNamed(t, res, "used"), entNamed(t, res, "unused"), entNamed(t, res, "exported")
	if !rs.Keep(used) {
		t.Error("used: called from the entry, should be kept")
	}
	if rs.Keep(unused) {
		t.Error("unused: called from nowhere, should not be kept")
	}
	if !rs.Keep(exported) {
		t.Error("exported: an export(C) root, should be kept")
	}
	if rs.Reason[used].Root != mir.RootNone || rs.Reason[used].Caller == 0 {
		t.Errorf("used should be reached by a call edge, got %+v", rs.Reason[used])
	}
	if rs.Reason[exported].Root != mir.RootExport {
		t.Errorf("exported should be a RootExport, got %+v", rs.Reason[exported])
	}
}

func TestReachabilityReportIsJSON(t *testing.T) {
	res, prog := checkedReach(t)
	rs := mir.Reachable(prog)
	report := rs.Report(res)
	if len(report) == 0 {
		t.Fatal("report: expected at least the roots")
	}
	var sawExport bool
	for _, e := range report {
		if e.Name == "exported" && e.Root == "export" {
			sawExport = true
		}
	}
	if !sawExport {
		t.Errorf("report: no export-rooted entry for `exported`, got %+v", report)
	}
	if _, err := json.Marshal(report); err != nil {
		t.Fatalf("report did not marshal: %v", err)
	}
}

func TestReachablePathNamesEntryToOp(t *testing.T) {
	res, prog := checkedReach(t)
	rs := mir.Reachable(prog)
	used := entNamed(t, res, "used")
	path := rs.PathNames(res, used)
	if len(path) == 0 || path[len(path)-1] != "used" {
		t.Errorf("PathNames(used) = %v, want a chain ending in \"used\"", path)
	}
}

func TestReachableCountsAStdFunctionTakenAsAValue(t *testing.T) {
	src := `
import ffi from std/ffi
import mem from std/mem

let make = ffi.CString.new
let erase = mem.secureErase
print "taken"
`
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	rs := mir.Reachable(mir.Build(res))
	seen := map[string]bool{}
	for _, op := range rs.Ops {
		seen[mir.StdOpNames[op]] = true
	}
	for _, want := range []string{"CSTRING_NEW", "SECURE_ERASE"} {
		if !seen[want] {
			t.Errorf("a function value of a std primitive does not reach %s; reached %v", want, seen)
		}
	}
}
