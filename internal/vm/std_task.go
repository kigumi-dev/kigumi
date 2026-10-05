package vm

import "kigumi/internal/sem"

func (m *Machine) executorOf(v *obj) *executor {
	d := deref(v)
	ex, ok := d.data.(*executor)
	if d.k != kOpaque || !ok {
		m.abort("value is not an Executor")
	}
	return ex
}

func (m *Machine) taskOf(v *obj) *task {
	d := deref(v)
	t, ok := d.data.(*task)
	if d.k != kOpaque || !ok {
		m.abort("value is not a Task")
	}
	return t
}

func (m *Machine) registerTask() {
	h := m.host
	h["task.currentThreadExecutor"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		return mkOpaque("Executor", &executor{})
	}
	h["task.Executor.runToCompletion"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		return m.runToCompletion(m.executorOf(a[0]), retain(deref(a[1])))
	}
	h["task.spawn"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		return m.spawnTask(retain(deref(a[0])))
	}
	h["task.Task.cancel"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		m.requestCancel(m.taskOf(a[0]))
		return unitObj
	}
	h["task.cancelRequested"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		return mkBool(m.cur != nil && m.cur.cancelReq)
	}
}
