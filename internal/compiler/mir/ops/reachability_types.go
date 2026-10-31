package ops

// NativeType names one std package member a reachability root can look
// up directly, the way shellRuntimeFn resolves a hidden call target.
type NativeType struct{ Package, Name string }

// nativeRecordTypesForOp names, per op, the std/ffi and std/task record
// types rt_core.c builds by name (find_type/make_record).
var nativeRecordTypesForOp = func() map[StdOpId][]NativeType {
	m := map[StdOpId][]NativeType{}
	for _, op := range Registry.Ops {
		if len(op.Types) > 0 {
			m[StdOpId(op.ID)] = op.Types
		}
	}
	return m
}()

func RuntimeNamedTypes() map[string]bool {
	out := map[string]bool{"Option": true, "Result": true, "Map": true, "Ordering": true, "Range": true, "AllocError": true}
	for _, op := range Registry.Ops {
		for _, ref := range op.Types {
			out[ref.Name] = true
		}
	}
	return out
}

func NativeTypes(id StdOpId) []NativeType {
	return append([]NativeType(nil), nativeRecordTypesForOp[id]...)
}
