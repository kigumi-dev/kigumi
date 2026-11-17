package sem

// checkThreadBounds judges the Send and Sync bounds the bodies left pending, once the context fixpoint has added the
// provider bindings closures capture; captures are final from here on, so closure verdicts may be memoized.
func (r *Result) checkThreadBounds() {
	r.thread.settled = true
	checks := r.thread.checks
	r.thread.checks = nil
	for _, pt := range checks {
		if why := r.threadCheckWhy(pt.threadCheck, pt.file); why != "" {
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

func (tc threadCheck) crossings(file FileID) []crossing {
	d := tc.held
	switch {
	case d == nil:
		return []crossing{{site: tc.site, producer: tc.producer, at: callRef{file, tc.node}}}
	case d.escapeAt == 0 && !d.called:
		return []crossing{{strict: true}}
	}
	out := make([]crossing, 0, len(d.sites)+1)
	for i, s := range d.sites {
		out = append(out, crossing{site: s, producer: tc.producer, at: callRef{file, d.calls[i]}})
	}
	if d.escapeAt != 0 {
		out = append(out, crossing{escaped: true})
	}
	return out
}

// A function item captures nothing, and one that needs a context key cannot be a value at all (E1217), so what holds
// only function items meets either bound.
func (r *Result) threadCheckWhy(tc threadCheck, file FileID) string {
	sync := tc.con.Kind == CSync
	if len(tc.fnItems()) > 0 || sync && r.isSync(tc.arg) {
		return ""
	}
	if !sync && !r.isSend(tc.arg) {
		return r.closureThreadWhy(tc.arg, sync, crossing{strict: true})
	}
	for _, x := range tc.crossings(file) {
		if !sync && !r.leavesAlone(tc.arg, x) {
			return r.closureThreadWhy(tc.arg, sync, x)
		}
	}
	if sync {
		return r.closureThreadWhy(tc.arg, sync, crossing{strict: true})
	}
	return ""
}

// ThreadBoundWhy is why a value of type t, holding only the function items items when there are any, fails a Send
// bound (a Sync one with sync) when it leaves site's frame; empty when it meets it.
func (r *Result) ThreadBoundWhy(t TypeID, sync bool, items []EntityID, site EntityID) string {
	kind := CSend
	if sync {
		kind = CSync
	}
	return r.threadCheckWhy(threadCheck{arg: t, con: Constraint{Kind: kind}, flow: &fnFlow{items: items}, site: site}, 0)
}
