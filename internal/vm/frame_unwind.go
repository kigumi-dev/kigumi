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
		m.sweepRelease(v)
	}
}

// IsCopy memoizes into the checker's shared table, so this locks derived.
func (m *Machine) ownsNothing(t sem.TypeID) bool {
	m.sh.derived.Lock()
	defer m.sh.derived.Unlock()
	return m.r.IsCopy(t) || m.r.Types.Kind(t) == sem.KRef
}

func joinPanic(first, second *Panic) *Panic {
	return &Panic{Msg: first.Msg + "\npanic: " + second.Msg, Site: first.Site}
}

func (m *Machine) sweepRelease(v *obj) {
	base := len(m.frames)
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		p, ok := r.(*Panic)
		if !ok || p.Sandbox || m.sweepFault == nil {
			panic(r)
		}
		m.sweepFault = joinPanic(m.sweepFault, p)
		m.sweepFrames(base)
	}()
	m.release(v)
}

func (m *Machine) sweepFrames(base int) {
	for len(m.frames) > base {
		fr := m.frames[len(m.frames)-1]
		m.frames = m.frames[:len(m.frames)-1]
		for _, l := range m.unwindSet(fr) {
			v := fr.locals[l]
			if v == nil || m.ownsNothing(fr.f.Locals[l].Type) {
				continue
			}
			fr.locals[l] = nil
			m.sweepRelease(v)
		}
	}
}

// The plan reads the checker's shared type table, which IsCopy writes under derived.
func (m *Machine) unwindSet(fr *frame) []mir.LocalID {
	m.sh.derived.Lock()
	defer m.sh.derived.Unlock()
	return m.p.UnwindPlanOf(fr.f).SetAt(fr.blk, fr.pc)
}
