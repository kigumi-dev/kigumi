package reach_test

import (
	"testing"

	mirbuild "kigumi/internal/compiler/mir/build"
	opdef "kigumi/internal/compiler/mir/ops"
	mirreach "kigumi/internal/compiler/mir/reach"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/lang/token"
	"kigumi/internal/toolchain/contract/schema"
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
	res := sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mirbuild.Program(res)
	rs := mirreach.Reachable(prog.R, prog.Entry, prog.Funcs, prog.ByEnt)

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
	res := sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mirbuild.Program(res)
	rs := mirreach.Reachable(prog.R, prog.Entry, prog.Funcs, prog.ByEnt)

	cstring := res.PackageMember(res.PackageByPath("std/ffi"), "CString")
	if cstring == 0 {
		t.Fatal("could not resolve std/ffi.CString")
	}
	if !rs.KeepType(cstring) {
		t.Error("CString: ffi.CString.new is reachable, CString should get a descriptor")
	}
	if id, ok := opdef.StdOpIdOf("CSTRING_NEW"); !ok || opdef.Capabilities([]opdef.StdOpId{id})[0] != "ffi.cstring" {
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
	res := sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	built := mirbuild.Program(res)
	rs := mirreach.Reachable(built.R, built.Entry, built.Funcs, built.ByEnt)
	caps := opdef.Capabilities(rs.Ops)
	if len(caps) != 1 || caps[0] != "ffi.cstring" {
		t.Fatalf("a std function named as a value should still reach its capability, got %v", caps)
	}
}

func TestReachableRootsDropGlueOnlyForKeptTypes(t *testing.T) {
	const src = `
type Held = resource { pub id Int }
type Idle = resource { pub id Int }

fn Held.drop(move self) {}
fn Idle.drop(move self) {}

fn open() -> Held { Held { id: 1 } }

let h = open()
print "ok"
`
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	built := mirbuild.Program(res)
	rs := mirreach.Reachable(built.R, built.Entry, built.Funcs, built.ByEnt)
	held, idle := res.TypeDecl(entType(t, res, "Held")).Drop, res.TypeDecl(entType(t, res, "Idle")).Drop
	if held == 0 || idle == 0 {
		t.Fatalf("both types should declare a drop, got %d and %d", held, idle)
	}
	if !rs.Keep(held) || rs.Reason[held].Root != mirreach.RootDropGlue {
		t.Errorf("Held is constructed, so its drop glue should be a drop-glue root, got %+v", rs.Reason[held])
	}
	if rs.Keep(idle) {
		t.Error("Idle is never constructed, so its drop glue should not be emitted")
	}
}

func TestReachableKeepsNestedLayoutFields(t *testing.T) {
	const src = `
pub type Header layout(C) = { pub magic u32 }
pub type Table layout(C) = { pub header Header, pub value u64 }

export(C) {
    pub fn probeNested(t: *const Table) -> u64 { 0 }
}

fn main() -> Void {}
`
	main := &sem.Package{Path: "main"}
	main.Entry = syntax.Parse(token.NewFile("main/main.kg", []byte(src)))
	main.Files = []*syntax.Tree{main.Entry}
	res := sem.CheckWith(&sem.Module{Packages: append(semfixture.LoadStd(t, "../../../../std"), main)}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mirbuild.Program(res)
	rs := mirreach.Reachable(prog.R, prog.Entry, prog.Funcs, prog.ByEnt)

	header, table := entType(t, res, "Header"), entType(t, res, "Table")
	if !rs.KeepType(table) || !rs.KeepType(header) {
		t.Errorf("nested layout(C) descriptors: Table %v, Header %v", rs.KeepType(table), rs.KeepType(header))
	}
}
