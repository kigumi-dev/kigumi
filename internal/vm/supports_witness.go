package vm

import "kigumi/internal/sem"

func (m *Machine) mayWitness(req, cand sem.EntityID) bool {
	key := [2]sem.EntityID{req, cand}
	if ok, known := m.witnessed[key]; known {
		return ok
	}
	if m.witnessed == nil {
		m.witnessed = map[[2]sem.EntityID]bool{}
	}
	ok := m.r.MayWitness(req, cand)
	m.witnessed[key] = ok
	return ok
}
