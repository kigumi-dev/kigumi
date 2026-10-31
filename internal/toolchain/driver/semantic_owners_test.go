package driver_test

import (
	"encoding/json"
	"os"
	"sort"
	"strings"
	"testing"

	"kigumi/internal/backend/vm"
	"kigumi/internal/compiler/mir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/toolchain/contract/schema"
)

// ownerEntry is one row of the semantic-owners inventory: every EfStd
// declaration and which backend(s) answer it.
type ownerEntry struct {
	Key         string `json:"key"` // "std/"-stripped package[.owner].name
	Signature   string `json:"signature"`
	HasBody     bool   `json:"has_body"`    // a Kigumi reference body exists
	Kind        string `json:"kind"`        // "primitive", "provider", or "" (not in the std op registry)
	VM          string `json:"vm"`          // "implemented" or "gap"
	Native      string `json:"native"`      // "implemented" or "gap"
	Accelerated bool   `json:"accelerated"` // HasBody and some backend also answers it directly
	Status      string `json:"status"`      // "ok", or "legacy-gap" with Reason set
	Reason      string `json:"reason,omitempty"`
}

// vmFFIGap lists bodiless FFI/dynamic-loader declarations the VM does not
// implement: raw foreign memory and symbol loading are native-only.
var vmFFIGap = map[string]bool{
	"ffi.CString.new": true, "ffi.CString.ptr": true, "ffi.CString.drop": true, "ffi.offset": true,
	"ffi.string": true, "ffi.stringChecked": true, "ffi.stringLossy": true,
	"ffi.readU8": true, "ffi.readI32": true, "ffi.readI64": true, "ffi.readF64": true, "ffi.writeU8": true,
	"ffi.writeI32": true, "ffi.writeI64": true, "ffi.writeF64": true, "ffi.bytesFrom": true, "ffi.bytesPtr": true,
	"ffi.readPtr": true, "ffi.writePtr": true, "ffi.secureEraseRaw": true,
	"os.Host.dl": true, "dl.Symbol.call": true,
}

// nativeLowered lists the primitives llgen expands at the call site
// instead of a runtime entry.
var nativeLowered = map[string]bool{
	"ffi.sizeOf": true, "ffi.alignOf": true, "ffi.readRecord": true, "ffi.writeRecord": true,
	"ffi.CValue.new": true, "ffi.CValue.ptr": true, "ffi.CValue.get": true, "ffi.CValue.set": true, "ffi.CValue.drop": true,
	"ffi.readU8": true, "ffi.readI32": true, "ffi.readI64": true, "ffi.readF64": true, "ffi.readPtr": true,
	"ffi.writeU8": true, "ffi.writeI32": true, "ffi.writeI64": true, "ffi.writeF64": true, "ffi.writePtr": true,
	"ffi.null": true, "ffi.isNull": true, "ffi.cast": true, "ffi.toConst": true, "ffi.toMut": true, "ffi.offset": true,
	"error.as": true, "mem.volatileRead": true, "mem.volatileWrite": true, "mem.volatileReadAt": true, "mem.volatileWriteAt": true,
	"mem.atomicLoad": true, "mem.atomicStore": true, "mem.atomicExchange": true, "mem.atomicCompareExchange": true, "mem.atomicAdd": true,
	"mem.atomicSub": true, "mem.atomicAnd": true, "mem.atomicOr": true, "mem.atomicXor": true,
}

// TestSemanticOwnersFresh diffs testdata/semantic_owners.json against std
// (UPDATE=1 regenerates it) and fails a bodiless declaration with no backend.
func TestSemanticOwnersFresh(t *testing.T) {
	res := sem.CheckWith(&sem.Module{Packages: testkit.LoadStd(t, "../../../std")}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	prog := mir.Build(res)

	var entries []ownerEntry
	for id := 1; id < len(res.Entities); id++ {
		fn := sem.EntityID(id)
		e := res.Entity(fn)
		if e.Kind != sem.EntFn || e.Flags&sem.EfStd == 0 || e.File == 0 || e.Name == "branch" {
			continue
		}
		info := res.Fn(fn)
		if info.Abi != "" || res.Entity(e.Parent).Kind == sem.EntInterface {
			continue
		}
		key := mir.StdKey(res, fn, true)
		hasBody := info.Body != 0
		kind := ""
		if _, prov, ok := mir.ResolveStdOp(res, fn); ok {
			kind = "primitive"
			if prov != 0 {
				kind = "provider"
			}
		}
		vmOK := vm.Implements(prog, fn)
		nativeOK := nativeCovers(key)
		status, reason := "ok", ""
		if !hasBody {
			if !vmOK && vmFFIGap[key] {
				status, reason = "legacy-gap", "ffi/dl: native-only; kigumi build diagnoses it on the VM"
			} else if !vmOK {
				status = "missing-vm"
				t.Errorf("%s: bodiless and the VM has no implementation", key)
			} else if !nativeOK {
				status = "missing-native"
				t.Errorf("%s: bodiless and native has no implementation", key)
			}
		}
		entries = append(entries, ownerEntry{
			Key: key, Signature: res.TypeString(info.Sig), HasBody: hasBody, Kind: kind,
			VM: gapOrOK(vmOK), Native: gapOrOK(nativeOK),
			Accelerated: hasBody && (vmOK || nativeOK),
			Status:      status, Reason: reason,
		})
	}
	// Overloads share a key; a stable sort keeps declaration order.
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].Key < entries[j].Key })
	if len(entries) < 40 {
		t.Fatalf("only %d std entries found; std not loaded as std?", len(entries))
	}

	path := "../../../testdata/semantic_owners.json"
	gotJSON, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("UPDATE") == "1" {
		if err := os.WriteFile(path, append(gotJSON, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v (run with UPDATE=1 to generate it)", path, err)
	}
	if string(want) != string(gotJSON)+"\n" {
		t.Errorf("%s is stale (run with UPDATE=1 to regenerate)", path)
	}
}

func gapOrOK(ok bool) string {
	if ok {
		return "implemented"
	}
	return "gap"
}

// isConversion matches a numeric `toX` method, lowered to rt_convert
// instead of a runtime table entry.
func isConversion(key string) bool {
	return strings.HasPrefix(key, "prelude.") && (strings.Contains(key, ".to") || strings.HasSuffix(key, ".truncate")) && !strings.HasPrefix(key, "prelude.String")
}

// nativeCovers is covered if the registry resolves it, llgen lowers it, or it is a numeric conversion.
func nativeCovers(key string) bool {
	if _, ok := opdef.StdOpFor(key); ok {
		return true
	}
	return nativeLowered[key] || isConversion(key)
}
