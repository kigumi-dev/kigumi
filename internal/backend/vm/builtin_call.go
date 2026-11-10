package vm

// callBuiltin runs a builtin through builtinTable, the one place SecretSan and the trace see its arguments.
func (m *Machine) callBuiltin(name string, args []*obj) *obj {
	if !m.opts.SecretSan && m.trace == nil {
		return m.builtin(name, args)
	}
	b := builtinTable[name]
	if b.out {
		m.checkHostIO(args, name)
	}
	m.argSinks(name, b.args, args)
	return m.labelResult(b.result, b.args, args, m.builtin(name, args))
}
