package mir

import "kigumi/internal/sem"

// nativeTypeRef names one std package member a reachability root can look
// up directly, the way shellRuntimeFn resolves a hidden call target.
type nativeTypeRef struct{ pkg, name string }

// nativeRecordTypeNames names, per StdOpNames entry, the std/ffi and
// std/task record types rt_core.c builds by name (find_type/make_record).
var nativeRecordTypeNames = map[string][]nativeTypeRef{
	"CSTRING_NEW":          {{"std/ffi", "CString"}},
	"CSTRING_PTR":          {{"std/ffi", "CString"}},
	"CSTRING_DROP":         {{"std/ffi", "CString"}},
	"PIN_NEW":              {{"std/ffi", "Pin"}},
	"PIN_PTR":              {{"std/ffi", "Pin"}},
	"PIN_WITH":             {{"std/ffi", "Pin"}},
	"PIN_DROP":             {{"std/ffi", "Pin"}},
	"PIN_RELEASE":          {{"std/ffi", "Pin"}},
	"PIN_RECLAIM":          {{"std/ffi", "Pin"}},
	"HANDLE_TABLE_NEW":     {{"std/ffi", "HandleTable"}},
	"HANDLE_TABLE_INSERT":  {{"std/ffi", "FfiHandle"}},
	"HANDLE_TABLE_WITH":    {{"std/ffi", "HandleTable"}},
	"HANDLE_TABLE_REMOVE":  {{"std/ffi", "HandleTable"}},
	"HANDLE_TABLE_RECLAIM": {{"std/ffi", "FfiHandle"}, {"std/ffi", "HandleFault"}},
	"HANDLE_TABLE_CLOSE":   {{"std/ffi", "HandleTableBusy"}},
	"HANDLE_TABLE_DROP":    {{"std/ffi", "HandleTable"}},
	"HANDLE_RAW":           {{"std/ffi", "RawHandle"}},
	"HANDLE_DROP":          {{"std/ffi", "HandleTable"}},
	"BLOCKING_POOL_NEW":    {{"std/task", "BlockingError"}},
	"BLOCKING_POOL_RUN":    {{"std/task", "BlockingError"}},
}

// nativeRecordTypesForOp is nativeRecordTypeNames keyed by StdOpId, built
// once since StdOpIdOf needs the package-level StdOpNames index.
var nativeRecordTypesForOp = func() map[StdOpId][]nativeTypeRef {
	m := make(map[StdOpId][]nativeTypeRef, len(nativeRecordTypeNames))
	for name, refs := range nativeRecordTypeNames {
		if id, ok := StdOpIdOf(name); ok {
			m[id] = refs
		}
	}
	return m
}()

// baseTypeEnts are types rt_core.c's rt_init reaches by name with no static
// call edge to see; omitting one turns a find_type miss into a NULL deref.
func baseTypeEnts(r *sem.Result) []sem.EntityID {
	prelude := r.PackageByPath("std/prelude")
	return []sem.EntityID{
		r.Types.OptionEnt(), r.Types.ResultEnt(), r.MapEntity(),
		r.PackageMember(prelude, "Ordering"),
		r.PackageMember(prelude, "Range"),
		r.PackageMember(prelude, "AllocError"),
	}
}

// Names rt_core.c looks up by name: always-kept base types plus those an op's record construction needs.
func RuntimeNamedTypes() map[string]bool {
	out := map[string]bool{"Option": true, "Result": true, "Map": true, "Ordering": true, "Range": true, "AllocError": true}
	for _, refs := range nativeRecordTypeNames {
		for _, ref := range refs {
			out[ref.name] = true
		}
	}
	return out
}
