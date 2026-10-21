package vm

func (p *task) addKid(t *task) {
	t.kidAt = len(p.kids)
	p.kids = append(p.kids, t)
	p.openKids++
}

func (m *Machine) cancelKids(t *task) {
	for _, k := range t.kids {
		if k != nil {
			m.cancelLocked(k)
		}
	}
}

// forgetKid drops an awaited child from its parent's scope, releasing its frame now instead of at the parent's end. Runs under sched.
// The slot becomes a nil hole so the other kids keep their order without a copy.
func (m *Machine) forgetKid(t *task) {
	p := t.parent
	if p == nil || t.kidAt >= len(p.kids) || p.kids[t.kidAt] != t {
		return
	}
	p.kids[t.kidAt] = nil
	p.goneKids++
	m.endTask(t)
	if p.goneKids*2 > len(p.kids) {
		p.compactKids()
	}
}

func (p *task) compactKids() {
	live := p.kids[:0]
	for _, k := range p.kids {
		if k != nil {
			k.kidAt = len(live)
			live = append(live, k)
		}
	}
	clear(p.kids[len(live):])
	p.kids, p.goneKids = live, 0
}

// notifyScope runs as t turns terminal: a panic nobody awaits cancels the parent, and with it every sibling.
func (m *Machine) notifyScope(t *task) {
	p := t.parent
	if p == nil {
		return
	}
	p.openKids--
	if t.state == taskFaulted && t.waiter == nil {
		m.cancelLocked(p)
	}
	if p.closing {
		p.ex.cv.Broadcast()
	}
}

func (m *Machine) checkOpen() {
	if m.cur.closing {
		m.abort("a task cannot spawn a child once its scope is closing; spawn it before the task returns, fails or is cancelled")
	}
}
