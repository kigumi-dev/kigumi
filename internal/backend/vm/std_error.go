package vm

import (
	"kigumi/internal/compiler/mir/instance"
	"kigumi/internal/compiler/sem"
)

func (m *Machine) registerError() {
	m.host["error.as"] = func(m *Machine, fn sem.EntityID, a []*obj) *obj {
		var opt sem.TypeID
		switch {
		case len(a) == 2:
			opt, a = sem.TypeID(deref(a[0]).i), a[1:]
		case m.siteRt != 0 && len(m.site.Rt) > 0:
			opt = m.siteRt
		default:
			opt, _ = m.r.Types.IsOption(m.siteFn.Locals[m.site.Dst].Type)
			opt = m.staticType(opt)
		}
		want, test := m.r.Types.Node(opt).Ent, instance.TestExact
		ctxEnt := m.r.LangItem("Context")
		cur := deref(a[0])
		owned := false
		for {
			if cur.k != kBox {
				break
			}
			et := m.typeEnt(cur.dynType())
			if m.holdsTest(test, want, opt, cur.dynType()) {
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
