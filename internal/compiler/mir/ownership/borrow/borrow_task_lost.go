package borrow

// closeDetached detaches what went into each lost Task, transitively.
func (r *taskRun) closeDetached() {
	for changed := true; changed; {
		changed = false
		for s, in := range r.inside {
			if add := in.minus(r.detached); r.lost(s) && !add.empty() {
				r.detached, changed = r.detached.or(add), true
			}
		}
	}
}

func (r *taskRun) lost(s int) bool { return r.dropped.has(s) || r.detached.has(s) }

// within is what Task s holds, through the Tasks that went into it.
func (r *taskRun) within(s int) srcSet {
	out := r.inside[s]
	for changed := true; changed; {
		changed = false
		out.each(func(t int) {
			if add := r.inside[t].minus(out); t != s && !add.empty() {
				out, changed = out.or(add), true
			}
		})
	}
	return out
}
