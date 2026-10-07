package mir_test

import (
	"testing"

	"kigumi/internal/mir"
	"kigumi/internal/schema"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
	"kigumi/internal/testkit"
	"kigumi/internal/token"
)

const reachTypeSrc = `
type Used = { n: Int }
type Unused = { n: Int }

fn build() -> Used { Used { n: 1 } }

let _ = build()
print "ok"
`

func entType(t *testing.T, r *sem.Result, name string) sem.EntityID {
	t.Helper()
	for id := 1; id < len(r.Entities); id++ {
		if e := r.Entity(sem.EntityID(id)); e.Kind == sem.EntType && e.Name == name {
			return sem.EntityID(id)
		}
	}
	t.Fatalf("no EntType named %q", name)
	return 0
}

func TestReachableKeepsOnlyConstructedTypes(t *testing.T) {
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(reachTypeSrc)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mir.Build(res)
	rs := mir.Reachable(prog)

	used, unused := entType(t, res, "Used"), entType(t, res, "Unused")
	if !rs.KeepType(used) {
		t.Error("Used: built by a reachable function, should get a descriptor")
	}
	if rs.KeepType(unused) {
		t.Error("Unused: never constructed, should not get a descriptor")
	}
}

func TestReachableKeepsNativeRecordTypeForOp(t *testing.T) {
	const src = `import ffi from std/ffi

let s = "hi"
let c = ffi.CString.new(&s).unwrap()
print "ok"
`
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mir.Build(res)
	rs := mir.Reachable(prog)

	cstring := res.PackageMember(res.PackageByPath("std/ffi"), "CString")
	if cstring == 0 {
		t.Fatal("could not resolve std/ffi.CString")
	}
	if !rs.KeepType(cstring) {
		t.Error("CString: ffi.CString.new is reachable, CString should get a descriptor")
	}
	if id, ok := mir.StdOpIdOf("CSTRING_NEW"); !ok || mir.Capabilities([]mir.StdOpId{id})[0] != "ffi.cstring" {
		t.Errorf("CSTRING_NEW should map to the ffi.cstring capability")
	}
}

func TestReachableCountsMentionedStdOp(t *testing.T) {
	const src = `import ffi from std/ffi

let make = ffi.CString.new
print "ok"
`
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	rs := mir.Reachable(mir.Build(res))
	caps := mir.Capabilities(rs.Ops)
	if len(caps) != 1 || caps[0] != "ffi.cstring" {
		t.Fatalf("a std function named as a value should still reach its capability, got %v", caps)
	}
}

func TestReachableRootsDropGlueOnlyForKeptTypes(t *testing.T) {
	const src = `
type Held = resource { pub id Int }
type Idle = resource { pub id Int }

fn Held.drop(move self) -> Unit {}
fn Idle.drop(move self) -> Unit {}

fn open() -> Held { Held { id: 1 } }

let h = open()
print "ok"
`
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(testkit.LoadStd(t, "../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	rs := mir.Reachable(mir.Build(res))
	held, idle := res.TypeDecl(entType(t, res, "Held")).Drop, res.TypeDecl(entType(t, res, "Idle")).Drop
	if held == 0 || idle == 0 {
		t.Fatalf("both types should declare a drop, got %d and %d", held, idle)
	}
	if !rs.Keep(held) || rs.Reason[held].Root != mir.RootDropGlue {
		t.Errorf("Held is constructed, so its drop glue should be a drop-glue root, got %+v", rs.Reason[held])
	}
	if rs.Keep(idle) {
		t.Error("Idle is never constructed, so its drop glue should not be emitted")
	}
}
