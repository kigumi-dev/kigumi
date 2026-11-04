package vm

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// Outside a task nothing catches a panic, and a root task keeps its own frames, so neither sweeps.
func (m *Machine) sweepsPanics() bool {
	return m.containing() && (m.cur == nil || m.cur.parent != nil)
}

func (m *Machine) heldFrame() *frame {
	var fr *frame
	if n := len(m.spare); n > 0 {
		fr = m.spare[n-1]
		m.spare = m.spare[:n-1]
	} else {
		fr = new(frame)
	}
	m.frames = append(m.frames, fr)
	return fr
}

func (m *Machine) holdInflight(f, body *midir.Func, params, args []*obj, bind bool) *frame {
	fr := m.heldFrame()
	if m.ownsLentEnv(f, body) {
		first, end := m.envSpan(f, len(params), len(args), bind)
		for i := first; i < end; i++ {
			if m.lentEnv(f, i) {
				fr.held = append(fr.held, params[i])
			}
		}
	}
	for i := len(args) - 1; i >= 0; i-- {
		fr.held = append(fr.held, args[i])
	}
	return fr
}

func (m *Machine) settleInflight(fr *frame) *Panic {
	m.frames = m.frames[:len(m.frames)-1]
	var fault *Panic
	for _, v := range fr.held {
		fault = m.releaseChild(v, fault)
	}
	clear(fr.held)
	fr.held = fr.held[:0]
	m.spare = append(m.spare, fr)
	return fault
}

func (m *Machine) sweepHeld(fr *frame) {
	for _, v := range fr.held {
		m.sweepRelease(v)
	}
	clear(fr.held)
	fr.held = fr.held[:0]
}

func (m *Machine) releaseInvoke(f, body *midir.Func, params, args []*obj, bind bool) {
	if m.ownsLentEnv(f, body) {
		first, end := m.envSpan(f, len(params), len(args), bind)
		for i := first; i < end; i++ {
			if m.lentEnv(f, i) {
				m.release(params[i])
			}
		}
	}
	for i := len(args) - 1; i >= 0; i-- {
		m.release(args[i])
	}
}

func (m *Machine) ownsLentEnv(f, body *midir.Func) bool {
	return body == f || m.r.Entity(f.Ent).Kind != sem.EntClosure
}

func (m *Machine) lentEnv(f *midir.Func, i int) bool {
	kind := m.r.Entity(f.Ent).Kind
	return kind == sem.EntClosure && m.p.CaptureLent(f, i) || kind == sem.EntFn && m.p.ParamLent(f.Ent, i)
}

func (m *Machine) envSpan(f *midir.Func, nparams, nargs int, bind bool) (first, end int) {
	kind := m.r.Entity(f.Ent).Kind
	if nparams == nargs || kind != sem.EntFn && kind != sem.EntClosure {
		return 0, 0
	}
	first = midir.BoundParam(bind, nargs, 0)
	return first, first + nparams - nargs
}
