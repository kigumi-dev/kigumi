package sem

// checkThreadBounds judges the Send and Sync bounds the bodies left pending, once the context fixpoint has added the
// provider bindings closures capture; captures are final from here on, so closure verdicts may be memoized.
func (r *Result) checkThreadBounds() {
	r.thread.settled = true
	checks := r.thread.checks
	r.thread.checks = nil
	for _, pt := range checks {
		if why := r.threadCheckWhy(pt.threadCheck); why != "" {
			r.errAt(pt.file, pt.node, cConstraintUnsatisfied, pt.label, pt.text, why)
		}
	}
}

// fnItems are the function items that alone reach the check's parameter; a value only counts when every use of it
// is a direct call.
func (tc threadCheck) fnItems() []EntityID {
	if d := tc.held; d != nil {
		if d.escapeAt != 0 || !d.called {
			return nil
		}
		return d.flows[tc.param].itemsOnly()
	}
	return tc.flow.itemsOnly()
}

// A function item captures nothing, and one that needs a context key cannot be a value at all (E1217), so what holds
// only function items meets either bound.
func (r *Result) threadCheckWhy(tc threadCheck) string {
	sync := tc.con.Kind == CSync
	switch {
	case len(tc.fnItems()) > 0:
		return ""
	case sync && r.isSync(tc.arg), !sync && r.isSend(tc.arg) && r.leavesAlone(tc.arg, tc.site):
		return ""
	}
	return r.closureThreadWhy(tc.arg, sync, tc.site)
}

// ThreadBoundWhy is why a value of type t, holding only the function items items when there are any, fails a Send
// bound (a Sync one with sync) when it leaves site's frame; empty when it meets it.
func (r *Result) ThreadBoundWhy(t TypeID, sync bool, items []EntityID, site EntityID) string {
	kind := CSend
	if sync {
		kind = CSync
	}
	return r.threadCheckWhy(threadCheck{arg: t, con: Constraint{Kind: kind}, flow: &fnFlow{items: items}, site: site})
}
