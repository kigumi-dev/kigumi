package sem

// checkInterruptSignature rejects what the non-naked wrapper (emit_cshim_export.go) can't lower without boxing/releasing.
func (r *Result) checkInterruptSignature(f FileID, e *Entity, info *FnInfo) {
	if info.Export != "interrupt" || info.Naked {
		return
	}
	if len(info.Params) > 0 || info.SelfParam != 0 {
		r.errAt(f, e.Node, cInterruptArgs, e.Name)
	}
	if r.Types.Node(info.Sig).Elem != TyUnit {
		r.errAt(f, e.Node, cInterruptReturn, e.Name)
	}
}
