package vm

import (
	"slices"
	"strings"
)

// closeScopeAt runs before the cleanups of an async body's exit, and closes the scope only when that body is the task's own.
func (m *Machine) closeScopeAt(fr *frame) {
	t := m.cur
	if t == nil || t.fut == nil {
		return
	}
	if fu, ok := deref(t.fut).opaqueData().(*future); !ok || fu.co == nil || &fu.co.fr != fr {
		return
	}
	m.closeScopeNow(t)
}

// closeOnPanic closes the scope before the unwinding drops run; a stack overflow has no room left to run the children.
func (m *Machine) closeOnPanic(p *Panic) {
	t := m.cur
	if t == nil || t.closing || p.Sandbox || m.sweepFault != nil || m.joinDepth > 0 || strings.HasPrefix(p.Msg, "stack overflow") {
		return
	}
	m.closeScopeNow(t)
}

// maxCloseNesting bounds the scopes closed inside one another on a lane; deeper ones join their children after their own cleanups, as the native C stack would not hold them.
const maxCloseNesting = 64

// closeScopeNow cancels the children still running and drives them to a terminal state before t's own cleanups run.
func (m *Machine) closeScopeNow(t *task) {
	m.sh.sched.Lock()
	if t.closing {
		m.sh.sched.Unlock()
		return
	}
	t.closing = true
	m.cancelKids(t)
	m.sh.sched.Unlock()
	if m.closeDepth >= maxCloseNesting {
		return
	}
	m.closeDepth++
	defer func() { m.closeDepth-- }()
	curFn, curInst, site, siteFn, inflight := m.curFn, m.curInst, m.site, m.siteFn, m.inflight
	m.sh.sched.Lock()
	m.sh.closeGen++
	mark := m.sh.closeGen
	m.sh.sched.Unlock()
	for m.driveDescendant(t, mark) {
	}
	m.curFn, m.curInst, m.site, m.siteFn, m.inflight = curFn, curInst, site, siteFn, inflight
}

// driveDescendant runs one queued descendant of t on this lane; only descendants run here, since they are all cancelled and wait on nothing outside t.
func (m *Machine) driveDescendant(t *task, mark uint64) bool {
	m.sh.sched.Lock()
	if t.openKids == 0 {
		m.sh.sched.Unlock()
		return false
	}
	ex := t.ex
	if f := ex.failed; f != nil {
		m.sh.sched.Unlock()
		m.rethrow(f)
	}
	d := takeDescendant(&ex.lane[m.curLane], t, mark)
	if d == nil {
		d = takeDescendant(&ex.pool, t, mark)
	}
	if d == nil {
		if ex.workers > 0 || ex.external > 0 {
			ex.cv.Wait()
			m.sh.sched.Unlock()
			return true
		}
		m.sh.sched.Unlock()
		m.abort("every task on this executor is waiting for something that cannot happen")
	}
	ex.running++
	m.sh.sched.Unlock()
	m.runTask(d, m.curLane)
	m.sh.sched.Lock()
	ex.running--
	ex.cv.Broadcast()
	m.sh.sched.Unlock()
	return true
}

func takeDescendant(k *taskQueue, t *task, mark uint64) *task {
	for i, d := range k.q {
		if descends(d, t, mark) {
			if i == 0 {
				k.q = k.q[1:]
			} else {
				k.q = slices.Delete(k.q, i, i+1)
			}
			d.queued = false
			return d
		}
	}
	return nil
}

// descends marks the tasks it walks through with this pass's mark, so a deep tree is walked once per pass.
func descends(d, t *task, mark uint64) bool {
	for p := d.parent; p != nil; p = p.parent {
		if p == t || p.closeMark == mark {
			for q := d; q != p; q = q.parent {
				q.closeMark = mark
			}
			return true
		}
	}
	return false
}
