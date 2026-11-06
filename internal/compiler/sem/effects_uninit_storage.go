package sem

// reportEdge names Uninit or BoundedArray when a noalloc break comes from their storage.
func (r *Result) reportEdge(e EffectEdge, mod Effects) {
	if mod == EffNoalloc && r.reportStorageAlloc(e) {
		return
	}
	r.reportContractEdge(e, mod)
}

// reportStorageAlloc reports e under the storage code when it calls into std/mem's Uninit or std/array's BoundedArray.
func (r *Result) reportStorageAlloc(e EffectEdge) bool {
	if e.Kind != EdgeCall {
		return false
	}
	typ := r.storageFn(e.Target)
	if typ == "" {
		return false
	}
	r.errAt(e.File, e.Node, cUninitStorageAlloc, r.Entities[e.Target].Name, typ)
	return true
}

// storageFn names the std/mem type whose heap storage fn needs: Uninit for `uninit`, BoundedArray for its own functions.
func (r *Result) storageFn(fn EntityID) string {
	if fn == 0 || r.Entities[fn].Kind != EntFn || r.Entities[fn].Flags&EfStd == 0 {
		return ""
	}
	if owner := r.Fn(fn).Owner; owner != 0 && owner == r.langItem(0, 0, "BoundedArray") {
		return "BoundedArray"
	}
	if r.Fn(fn).Owner == 0 && r.Entities[fn].Name == "uninit" && r.Packages[r.Entities[fn].Pkg].Path == "std/mem" {
		return "Uninit"
	}
	return ""
}
