package vm

import "kigumi/internal/sem"

func (m *Machine) registerError() {
	m.host["error.as"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		opt, _ := m.r.Types.IsOption(m.siteFn.Locals[m.site.Dst].Type)
		want := m.r.Types.Node(opt).Ent
		ctxEnt := m.r.LangItem("Context")
		cur := deref(a[0])
		owned := false
		for {
			if cur.k != kBox {
				break
			}
			et := m.typeEnt(cur.dynType())
			if et == want {
				v := m.copiedResult(m.stdKey(fn), cur.inner)
				if owned {
					m.release(cur)
				}
				return v
			}
			if ctxEnt == 0 || et != ctxEnt {
				break
			}
			idx := m.r.Field(m.r.FindField(ctxEnt, "wrapped")).Index
			next := deref(m.field(cur, idx))
			if owned {
				m.release(cur)
			}
			cur, owned = next, true
		}
		if owned {
			m.release(cur)
		}
		return m.none()
	}
}
