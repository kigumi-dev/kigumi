package mir

// nativeTypeRef names one std package member a reachability root can look
// up directly, the way shellRuntimeFn resolves a hidden call target.
type nativeTypeRef struct{ pkg, name string }

// nativeRecordTypesForOp names, per op, the std/ffi and std/task record
// types rt_core.c builds by name (find_type/make_record).
var nativeRecordTypesForOp = func() map[StdOpId][]nativeTypeRef {
	m := map[StdOpId][]nativeTypeRef{}
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
			out[ref.name] = true
		}
	}
	return out
}
