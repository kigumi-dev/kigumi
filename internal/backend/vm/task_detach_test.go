package vm

import "testing"

func liveKids(p *task) []*task {
	var out []*task
	for _, k := range p.kids {
		if k != nil {
			out = append(out, k)
		}
	}
	return out
}

func TestDetachedKidLeavesScopeOnceFinished(t *testing.T) {
	m := &Machine{sh: newShared()}
	ex := m.newExecutor(0)
	parent := &task{ex: ex, state: taskSuspended}
	done := &task{ex: ex, parent: parent, pex: ex, state: taskCompleted}
	running := &task{ex: ex, parent: parent, pex: ex, state: taskRunning}
	faulted := &task{ex: ex, parent: parent, pex: ex, state: taskFaulted}
	kept := &task{ex: ex, parent: parent, pex: ex, state: taskCompleted}
	for _, k := range []*task{done, running, faulted, kept} {
		parent.addKid(k)
	}
	parent.openKids = 1

	m.detachTask(done)
	m.detachTask(running)
	m.detachTask(faulted)
	if got := liveKids(parent); len(got) != 3 || got[0] != running || got[1] != faulted || got[2] != kept {
		t.Fatalf("after the handles went the parent keeps %d kids, want the running, faulted and still-held ones", len(got))
	}

	ex.live = 1
	m.beginFinish(running, taskCompleted)
	if got := liveKids(parent); len(got) != 2 || got[0] != faulted || got[1] != kept {
		t.Fatalf("a detached kid that finished stayed in its parent's scope: %d kids left", len(got))
	}
}
