package vm

import "kigumi/internal/sem"

func (m *Machine) callRequirementValue(fv *obj, args []*obj) *obj {
	env := fv.env.fields
	if fv.req.witness {
		recv := env[1]
		if fv.req.move {
			recv = m.copy(recv)
		}
		return m.callValue(env[0], append([]*obj{recv}, args...))
	}
	return m.callMethod(env[0], fv.req.name, args, sem.RecvSelf)
}
