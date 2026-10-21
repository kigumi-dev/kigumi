package vm

import "kigumi/internal/sem"

func (m *Machine) executorOf(v *obj) *executor {
	d := deref(v)
	ex, ok := d.opaqueData().(*executor)
	if d.k != kOpaque || !ok {
		m.abort("value is not an Executor")
	}
	return ex
}

func (m *Machine) taskOf(v *obj) *task {
	d := deref(v)
	t, ok := d.opaqueData().(*task)
	if d.k != kOpaque || !ok {
		m.abort("value is not a Task")
	}
	return t
}

// A thread pool is bounded and root-owned; a sandboxed run (comptime, build
// program) has no place to own one.
func (m *Machine) threadPool(workers int64) *obj {
	if m.opts.Sandbox {
		m.abort("a thread pool executor is not available here")
	}
	if workers < 1 {
		m.abort("a thread pool executor needs at least one worker")
	}
	if workers > 256 {
		m.abort("a thread pool executor is bounded to 256 workers")
	}
	e := m.newExecutor(int(workers))
	m.startWorkers(e)
	return mkOpaque("Executor", e)
}

func (m *Machine) registerTask() {
	h := m.host
	h["task.currentThreadExecutor"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		return mkOpaque("Executor", m.newExecutor(0))
	}
	h["task.threadPoolExecutor"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		return m.threadPool(deref(a[0]).i)
	}
	h["task.Executor.handoff"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		return m.handoffTask(m.executorOf(a[0]), retain(deref(a[1])))
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
	h["task.yieldNow"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		return mkOpaque("Future", &future{yield: 1})
	}
	h["task.cancelRequested"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		return mkBool(m.cur != nil && m.cur.cancelReq.Load())
	}
}
