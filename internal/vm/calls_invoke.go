package vm

import (
	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

// entryOf is what a boxed call runs: the all-boxed entry of a function with a direct signature.
func entryOf(f *mir.Func) *mir.Func {
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

// invoke is the adapter convention: the callee takes a retained reference
// to every argument, the environment supplies the captured leading
// parameters (or the bound trailing ones), and the argument array is
// released afterwards.
func (m *Machine) invoke(f *mir.Func, env *obj, bind bool, args []*obj) *obj {
	return m.invokeAs(f, entryOf(f), env, bind, args)
}

// invokeAs runs body, f itself or f's entry, under invoke's convention.
func (m *Machine) invokeAs(f, body *mir.Func, env *obj, bind bool, args []*obj) *obj {
	nenv := 0
	if env != nil {
		nenv = len(env.fields)
	}
	params := m.argSlice(nenv + len(args))[:0]
	if bind {
		for _, a := range args {
			params = append(params, retain(a))
		}
		for i := 0; i < nenv; i++ {
			params = append(params, retain(env.fields[i]))
		}
	} else {
		for i := 0; i < nenv; i++ {
			params = append(params, retain(env.fields[i]))
		}
		for _, a := range args {
			params = append(params, retain(a))
		}
	}
	r := m.call(body, params)
	if body == f || m.r.Entity(f.Ent).Kind != sem.EntClosure {
		m.releaseLentEnv(f, params, len(args), bind)
	}
	// Reverse, as a direct call's arguments drop and rt_core.c's invoke releases them.
	for i := len(args) - 1; i >= 0; i-- {
		m.release(args[i])
	}
	return r
}

// releaseLentEnv drops the references invoke took for captured parameters the
// callee only borrows, so the callee has nothing to release them.
func (m *Machine) releaseLentEnv(f *mir.Func, params []*obj, nargs int, bind bool) {
	kind := m.r.Entity(f.Ent).Kind
	if len(params) == nargs || kind != sem.EntFn && kind != sem.EntClosure {
		return
	}
	first, end := 0, len(params)-nargs
	if bind {
		first, end = nargs, len(params)
	}
	for i := first; i < end; i++ {
		if kind == sem.EntClosure && m.p.CaptureLent(f, i) || kind == sem.EntFn && m.p.ParamLent(f.Ent, i) {
			m.release(params[i])
		}
	}
}
