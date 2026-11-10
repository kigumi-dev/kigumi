package vm_test

import (
	"testing"

	"kigumi/internal/backend/vm"
	mirbuild "kigumi/internal/compiler/mir/build"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

// vmGaps are the bodiless std functions the VM leaves to `kigumi build` that
// resolve to no registry op: the raw-FFI primitives llgen lowers at their call
// site. A gap that does resolve to an op is an op gap and belongs in
// vmGapReasons (op_manifest_test.go), which keeps the manifest and this test on
// one list.
var vmGaps = map[string]bool{
	"std/ffi.offset": true,
	"std/ffi.readU8": true, "std/ffi.readI32": true, "std/ffi.readI64": true, "std/ffi.readF64": true,
	"std/ffi.writeU8": true, "std/ffi.writeI32": true, "std/ffi.writeI64": true, "std/ffi.writeF64": true,
	"std/ffi.readPtr": true, "std/ffi.writePtr": true,
}

// TestStdPrimitives pins the contract between std and the VM: every
// bodiless std function has an implementation, except the listed gaps.
func TestStdPrimitives(t *testing.T) {
	res := checkSource(t, "print \"x\"\n")
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mirbuild.Program(res)
	n := 0
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
		key := res.Packages[e.Pkg].Path + "."
		if info.Owner != 0 {
			key += res.Entity(info.Owner).Name + "."
		}
		key += e.Name
		n++
		want := vmGaps[key]
		if opID, ok := opdef.StdOpFor(key); ok {
			want = vmGapReasons[opdef.StdOpNames[opID]] != ""
		}
		if vm.Implements(prog, fn) == want {
			t.Errorf("%s: implemented=%v, listed as a gap=%v", key, vm.Implements(prog, fn), want)
		}
	}
	for key := range vmGaps {
		if opID, ok := opdef.StdOpFor(key); ok {
			t.Errorf("%s resolves to op %s; declare the gap in vmGapReasons instead", key, opdef.StdOpNames[opID])
		}
	}
	if n < 40 {
		t.Fatalf("only %d bodiless std functions found", n)
	}
}
