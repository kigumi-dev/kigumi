package vm

// deref follows cells, as every runtime entry point does.
func deref(v *obj) *obj {
	for v != nil && v.k >= kCell {
		if v.k == kRawArray {
			v.expand()
			break
		}
		v = v.inner
	}
	return v
}

// derefRaw is deref that leaves a compact array compact, for the operations that read it in place.
func derefRaw(v *obj) *obj {
	for v != nil && v.k == kCell {
		v = v.inner
	}
	return v
}
