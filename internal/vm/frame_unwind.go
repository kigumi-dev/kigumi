package vm

import (
	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

func (m *Machine) dropOwnedLocals(f *mir.Func, locals []*obj) {
	for i := len(locals) - 1; i >= 0; i-- {
		v := locals[i]
		if v == nil {
			continue
		}
		l := f.Locals[i]
		if l.Ent == 0 || l.Borrowed || m.ownsNothing(l.Type) {
			continue
		}
		locals[i] = nil
		m.release(v)
	}
}

// IsCopy memoizes into the checker's shared table, so this locks derived.
func (m *Machine) ownsNothing(t sem.TypeID) bool {
	m.sh.derived.Lock()
	defer m.sh.derived.Unlock()
	return m.r.IsCopy(t) || m.r.Types.Kind(t) == sem.KRef
}
