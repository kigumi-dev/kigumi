package vm

import "sync/atomic"

// Go recursion depth of a free before the rest of the subtree moves to an explicit stack.
const freeDepthMax = 256

type freeFrame struct {
	v      *obj
	fields []*obj
	single *obj
	i      int
	step   int
	borrow bool
	owned  bool
	fault  *Panic
}

// freeDeep visits children in the order freeAt does, so drop hooks run in the same order.
func (m *Machine) freeDeep(root *obj) {
	stack := []freeFrame{m.openFree(root)}
	for len(stack) > 0 {
		top := &stack[len(stack)-1]
		if c, ok := top.nextChild(m); ok {
			if frame, push := m.childFrame(c); push {
				stack = append(stack, frame)
			}
			continue
		}
		fault := m.closeFree(top)
		stack = stack[:len(stack)-1]
		switch {
		case fault == nil:
		case len(stack) == 0:
			m.raiseJoined(fault)
		default:
			parent := &stack[len(stack)-1]
			parent.fault = m.callJoined(parent.fault, func() { m.raiseJoined(fault) })
		}
	}
}

func (m *Machine) childFrame(c *obj) (freeFrame, bool) {
	if c == nil {
		return freeFrame{}, false
	}
	if rc := atomic.LoadInt32(&c.rc); rc <= 0 {
		if rc == 0 && c.k == kFree {
			noteFreedRelease()
		}
		return freeFrame{}, false
	}
	if atomic.AddInt32(&c.rc, -1) != 0 {
		return freeFrame{}, false
	}
	switch c.k {
	case kInt, kFloat, kChar:
		m.free(c)
		return freeFrame{}, false
	}
	return m.openFree(c), true
}

func (m *Machine) openFree(v *obj) freeFrame {
	m.unchargeMemory(v)
	fr := freeFrame{v: v, step: 1}
	switch v.k {
	case kRecord:
		setImmortal(v)
		fr.fault = m.runDropHook(v, nil)
		atomic.StoreInt32(&v.rc, 0)
		fr.fields, fr.i, fr.step, fr.borrow = v.fields, len(v.fields)-1, -1, true
	case kVariant:
		fr.fields = v.fields
	case kArray:
		fr.fields, fr.i, fr.step = v.fields, len(v.fields)-1, -1
	case kBox, kCell:
		if v.k != kCell || !v.b {
			fr.single = v.inner
		}
	case kClosure:
		fr.single = v.cenv()
	case kOpaque:
		if fr.single = m.opaqueOwned(v); fr.single != nil {
			fr.owned = true
		}
	}
	return fr
}

func (fr *freeFrame) nextChild(m *Machine) (*obj, bool) {
	for fr.i >= 0 && fr.i < len(fr.fields) {
		i := fr.i
		fr.i += fr.step
		if fr.borrow && m.borrowField(fr.v, i) {
			continue
		}
		return fr.fields[i], true
	}
	if fr.single != nil {
		c := fr.single
		fr.single = nil
		return c, true
	}
	return nil, false
}

func (m *Machine) closeFree(fr *freeFrame) *Panic {
	v := fr.v
	if v.k == kOpaque && !fr.owned {
		m.freeOpaqueAt(v, 0)
	}
	dropExtension(v)
	tracked := v.live
	*v = obj{k: kFree}
	if liveTrack.Load() {
		liveDropped(tracked)
	}
	return fr.fault
}
