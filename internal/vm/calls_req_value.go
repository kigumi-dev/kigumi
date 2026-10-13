package vm

func (m *Machine) callRequirementValue(fv *obj, args []*obj) *obj {
	env := fv.env.fields
	if fv.req.witness {
		return m.callValue(env[0], append([]*obj{env[1]}, args...))
	}
	return m.callMethod(env[0], fv.req.name, args, false)
}
