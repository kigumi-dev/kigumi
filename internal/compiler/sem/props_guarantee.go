package sem

// evalGuarantee reports whether t's guarantee (if any) grants prop, given
// the actual type arguments args applied to its own type parameters.
func (r *Result) evalGuarantee(info *TypeDeclInfo, args []TypeID, prop ugProp) bool {
	g := info.Guarantee
	if g == nil {
		return false
	}
	rule := g.Rules[ugIndex(prop)]
	if rule == nil {
		return false
	}
	for _, c := range rule.Conds {
		idx := -1
		for i, p := range info.Params {
			if p == c.Param {
				idx = i
				break
			}
		}
		if idx < 0 || idx >= len(args) {
			return false
		}
		arg := args[idx]
		if c.Need&ugCopyProp != 0 && !r.isCopy(arg) {
			return false
		}
		if c.Need&ugSendProp != 0 && !r.isSend(arg) {
			return false
		}
		if c.Need&ugSyncProp != 0 && !r.isSync(arg) {
			return false
		}
	}
	return true
}
