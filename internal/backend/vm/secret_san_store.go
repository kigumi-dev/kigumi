package vm

func (m *Machine) labelOf(v *obj) label {
	m.sh.shadowMu.Lock()
	l := m.sh.shadows[v]
	m.sh.shadowMu.Unlock()
	return l
}

// mergeLabel joins l into v's own label; a shape reaches only a kind that has one.
func (m *Machine) mergeLabel(v *obj, l label) {
	if v == nil {
		return
	}
	if !hasShape(v.k) {
		l = label{elem: l.elem}
	}
	if !l.tainted() {
		return
	}
	m.sh.shadowMu.Lock()
	if m.sh.shadows == nil {
		m.sh.shadows = map[*obj]label{}
	}
	m.sh.shadows[v] = m.sh.shadows[v].merge(l)
	m.sh.shadowMu.Unlock()
}

func (m *Machine) addLabel(v *obj, t shadow) { m.mergeLabel(v, label{elem: t}) }

// clearLabel forgets v's label, so a freed object cannot lend it to the next one at its address.
func (m *Machine) clearLabel(v *obj) {
	if v == nil {
		return
	}
	m.sh.shadowMu.Lock()
	delete(m.sh.shadows, v)
	m.sh.shadowMu.Unlock()
}

// chainLabel is v's label joined with the labels of the cells and boxes that wrap it.
func (m *Machine) chainLabel(v *obj) label {
	var l label
	if !m.opts.SecretSan {
		return l
	}
	for v != nil {
		l = l.merge(m.labelOf(v))
		switch v.k {
		case kCell, kBox:
			v = v.inner
		default:
			return l
		}
	}
	return l
}

func (m *Machine) taintOf(v *obj) shadow { return m.chainLabel(v).whole() }

func (m *Machine) shapeOf(v *obj) bool { return m.chainLabel(v).shape }

func (m *Machine) anyShape(args []*obj) bool {
	for _, a := range args {
		if m.shapeOf(a) {
			return true
		}
	}
	return false
}

func (m *Machine) joinTaint(args []*obj) shadow {
	var t shadow
	for _, a := range args {
		t = t.merge(m.taintOf(a))
	}
	return t
}

// deepTaint is the label of a use that reads everything reachable from args (display, equality, hashing, output).
func (m *Machine) deepTaint(args []*obj) shadow {
	var t shadow
	if !m.opts.SecretSan {
		return t
	}
	seen := map[*obj]bool{}
	var walk func(v *obj, depth int)
	walk = func(v *obj, depth int) {
		if v == nil || seen[v] {
			return
		}
		if depth > deepTaintMax {
			t = t.merge(secretCoarse)
			return
		}
		seen[v] = true
		t = t.merge(m.labelOf(v).whole())
		switch v.k {
		case kRecord, kVariant, kArray:
			for _, f := range v.fields {
				walk(f, depth+1)
			}
		case kCell, kBox:
			walk(v.inner, depth+1)
		}
	}
	for _, a := range args {
		walk(a, 0)
	}
	return t
}

// deepTaintMax bounds the walk; a deeper value is taken as secret rather than left unread.
const deepTaintMax = 256
