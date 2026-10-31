package sem

// applyAsmOpEdge folds an asm block into a summary: an unverified template is Untrusted, and a `sym`
// operand's function is merged like a call, except an export(C) wrapper, which may box its arguments.
func (r *Result) applyAsmOpEdge(s *OpSummary, e EffectEdge) {
	if e.Param == 0 {
		if r.isrAsmVerdict(e) != "" {
			s.Untrusted = true
		}
		return
	}
	if r.asmSymBoxes(e.Param) {
		setDirect(s, FactAlloc, e, false)
		return
	}
	mergeInto(s, r.opSummary(e.Param), e, false)
}

// asmSymBoxes reports a `sym` target reached through an export(C) wrapper that boxes an argument or result.
func (r *Result) asmSymBoxes(fn EntityID) bool {
	info := r.Fn(fn)
	if info.Export == "" || info.Export == "interrupt" || info.Naked {
		return false
	}
	sig := r.Types.Node(info.Sig)
	return len(sig.Args) != 0 || sig.Elem != TyUnit
}
