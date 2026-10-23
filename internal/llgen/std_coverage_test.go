package llgen_test

import (
	"strings"

	"kigumi/internal/sem"
)

// lowered lists the primitives llgen expands at the call site, so the
// runtime has no entry for them.
var lowered = map[string]bool{
	"ffi.sizeOf": true, "ffi.alignOf": true, "ffi.readRecord": true, "ffi.writeRecord": true,
	"ffi.CValue.new": true, "ffi.CValue.ptr": true, "ffi.CValue.get": true, "ffi.CValue.set": true, "ffi.CValue.drop": true,
	"ffi.readU8": true, "ffi.readI32": true, "ffi.readI64": true, "ffi.readF64": true, "ffi.readPtr": true,
	"ffi.writeU8": true, "ffi.writeI32": true, "ffi.writeI64": true, "ffi.writeF64": true, "ffi.writePtr": true,
	"ffi.null": true, "ffi.isNull": true, "ffi.cast": true, "ffi.toConst": true, "ffi.toMut": true, "ffi.offset": true,
	"error.as": true, "mem.volatileRead": true, "mem.volatileWrite": true, "mem.volatileReadAt": true, "mem.volatileWriteAt": true,
	"mem.atomicLoad": true, "mem.atomicStore": true, "mem.atomicExchange": true, "mem.atomicCompareExchange": true, "mem.atomicAdd": true,
	"mem.atomicSub": true, "mem.atomicAnd": true, "mem.atomicOr": true, "mem.atomicXor": true,
}

// conversionKey matches the numeric `toX` methods, which callEntity lowers
// to rt_convert.
func conversionKey(key string) bool {
	return strings.HasPrefix(key, "prelude.") && (strings.Contains(key, ".to") || strings.HasSuffix(key, ".truncate")) && !strings.HasPrefix(key, "prelude.String")
}

func primitiveKey(res *sem.Result, fn sem.EntityID) (string, bool) {
	e := res.Entity(fn)
	if e.Kind != sem.EntFn || e.Flags&sem.EfStd == 0 || e.File == 0 || e.Name == "branch" {
		return "", false
	}
	info := res.Fn(fn)
	if info.Body != 0 || info.Abi != "" || res.Entity(e.Parent).Kind == sem.EntInterface {
		return "", false
	}
	key := res.Packages[e.Pkg].Path + "."
	if info.Owner != 0 {
		key += res.Entity(info.Owner).Name + "."
	}
	return key + e.Name, true
}
