package vm

import "kigumi/internal/compiler/sem"

// exitSignal ends a run at its entry; a lane that sees it runs no program code on the way up.
type exitSignal struct{ code int }

func (e *exitSignal) Error() string { return "exit" }

func (m *Machine) registerExit() {
	m.host["os.Host.exit"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		if m.opts.DenyExit {
			m.abort("`host.exit` is not available under `kigumi test`, which runs in the same process as the runner")
		}
		panic(&exitSignal{code: int(deref(a[1]).i)})
	}
}

// exitStatus finds an ExitError in e or in the Context chain around it.
func (m *Machine) exitStatus(e *obj) (int, bool) {
	want := m.r.LangItem("ExitError")
	if want == 0 {
		return 0, false
	}
	ctxEnt := m.r.LangItem("Context")
	for cur := deref(e); cur.k == kBox; {
		et := m.typeEnt(cur.dynType())
		if et == want {
			idx := m.r.Field(m.r.FindField(want, "code")).Index
			return int(deref(m.field(cur, idx)).i), true
		}
		if ctxEnt == 0 || et != ctxEnt {
			break
		}
		cur = deref(m.field(cur, m.r.Field(m.r.FindField(ctxEnt, "wrapped")).Index))
	}
	return 0, false
}
