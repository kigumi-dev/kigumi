package vm

import (
	midir "kigumi/internal/compiler/mir/ir"
)

// entryOf is what a boxed call runs: the all-boxed entry of a function with a direct signature.
func entryOf(f *midir.Func) *midir.Func {
	if f.Entry != nil {
		return f.Entry
	}
	return f
}

// callDirectValue calls an unboxed closure straight into its body, as native does.
func (m *Machine) callDirectValue(fv *obj, args []*obj) *obj {
	fv = deref(fv)
	if fv.k != kClosure || fv.cfn() == nil || fv.creq() != nil || fv.async || fv.cstd() != "" {
		return m.callValue(fv, args)
	}
	return m.invokeAs(fv.cfn(), fv.cfn(), fv.cenv(), fv.bind, args)
}

// invoke is the adapter convention: the callee takes a retained reference to every parameter AppendValueParams lays
// out from the environment and the arguments, and the argument array is released afterwards.
func (m *Machine) invoke(f *midir.Func, env *obj, bind bool, args []*obj) *obj {
	return m.invokeAs(f, entryOf(f), env, bind, args)
}

// invokeAs runs body, f itself or f's entry, under invoke's convention.
func (m *Machine) invokeAs(f, body *midir.Func, env *obj, bind bool, args []*obj) *obj {
	var bound []*obj
	if env != nil {
		bound = env.fields
	}
	params := midir.AppendValueParams(m.argSlice(len(bound) + len(args))[:0], bind, bound, args)
	for _, p := range params {
		retain(p)
	}
	if !m.sweepsPanics() {
		r := m.call(body, params)
		m.releaseInvoke(f, body, params, args, bind)
		return r
	}
	rec := m.holdInflight(f, body, params, args, bind)
	r := m.call(body, params)
	m.raiseJoined(m.settleInflight(rec))
	return r
}

func envLen(fv *obj) int {
	if env := fv.cenv(); env != nil {
		return len(env.fields)
	}
	return 0
}
