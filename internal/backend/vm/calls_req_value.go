package vm

import "kigumi/internal/compiler/sem"

func (m *Machine) callRequirementValue(fv *obj, args []*obj) *obj {
	env := fv.cenv().fields
	if fv.creq().witness {
		recv := env[1]
		if fv.creq().move || deref(env[0]).async {
			recv = m.copy(recv)
		}
		return m.callValue(env[0], append([]*obj{recv}, args...))
	}
	return m.callMethod(env[0], fv.creq().name, args, sem.RecvSelf)
}
