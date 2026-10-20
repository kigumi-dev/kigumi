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

// A fn value is Copy yet its environment can own a resource, so Copy alone is not enough; CannotOwn can intern types, so this locks derived.
func (m *Machine) ownsNothing(t sem.TypeID) bool {
	m.sh.derived.Lock()
	defer m.sh.derived.Unlock()
	return m.r.CannotOwn(t) || m.r.Types.Kind(t) == sem.KRef
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
		if fr.f == nil {
			m.sweepHeld(fr)
			continue
		}
		for _, l := range m.unwindSet(fr) {
			v := fr.locals[l]
			if v == nil || m.ownsNothing(fr.f.Locals[l].Type) {
				continue
			}
			fr.locals[l] = nil
			m.sweepRelease(v)
		}
		if len(fr.pending) > 0 {
			m.sweepCrossings(fr)
		}
	}
}

// The plan reads the checker's shared type table, which IsCopy writes under derived.
func (m *Machine) unwindSet(fr *frame) []mir.LocalID {
	m.sh.derived.Lock()
	defer m.sh.derived.Unlock()
	return m.p.UnwindPlanOf(fr.f).SetAt(fr.blk, fr.pc)
}

// Outside a task a panic goes straight up, as in native where it is fatal.
func (m *Machine) releaseJoined(v *obj, fault *Panic) (out *Panic) {
	if !m.containing() || v == nil {
		m.release(v)
		return fault
	}
	return m.callJoined(fault, func() { m.release(v) })
}

func (m *Machine) containing() bool { return m.cur != nil || m.outerDepth > 0 }

func (m *Machine) recoverJoined(r any, fault *Panic, base int) *Panic {
	p, ok := r.(*Panic)
	if !ok || p.Sandbox {
		panic(r)
	}
	if fault != nil {
		p = joinPanic(fault, p)
	}
	outer := m.sweepFault
	m.sweepFault = p
	m.sweepFrames(base)
	out := m.sweepFault
	m.sweepFault = outer
	return out
}

func (m *Machine) raiseJoined(fault *Panic) {
	if fault != nil {
		panic(fault)
	}
}
