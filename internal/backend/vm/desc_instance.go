package vm

import (
	"kigumi/internal/compiler/mir/instance"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// canon is t's runtime identity; interning can grow the checker's table, so the lock covers it.
func (m *Machine) canon(t sem.TypeID) sem.TypeID {
	m.sh.derived.Lock()
	defer m.sh.derived.Unlock()
	return m.p.Canon.Of(m.r, t)
}

// siteType is the runtime type a box, type test or record names: the one in.Rt composed, else its static type's
// identity, cached on the instruction.
func (m *Machine) siteType(fr *frame, in *midir.Inst, ic *instCode, t sem.TypeID) sem.TypeID {
	if len(in.Rt) > 0 {
		return sem.TypeID(deref(fr.locals[in.Rt[0]]).i)
	}
	if ic != nil {
		if c := ic.canon.Load(); c != 0 {
			return sem.TypeID(c)
		}
	}
	c := m.staticType(t)
	if ic != nil {
		ic.canon.Store(uint32(c))
	}
	return c
}

// staticType is the identity of a type a site names without composing it, which must be one runtime type.
func (m *Machine) staticType(t sem.TypeID) sem.TypeID {
	if !instance.Concrete(m.r, t) {
		m.abort("internal error: a runtime type names parameters with no environment")
	}
	return m.canon(t)
}

// holdsTest compares a box's dynamic type dyn with pat, the identity a test of kind test names.
func (m *Machine) holdsTest(test instance.Test, ent sem.EntityID, pat, dyn sem.TypeID) bool {
	if test == instance.TestAny {
		return m.typeEnt(dyn) == ent
	}
	return dyn == pat
}

func (m *Machine) typeTestOf(in *midir.Inst, ic *instCode) instance.Test {
	if c := ic.typeTest.Load(); c != 0 {
		return instance.Test(c - 1)
	}
	t := instance.TypeTest(m.r, in.Type)
	ic.typeTest.Store(int32(t) + 1)
	return t
}

// descCall is the body and arguments a method-table slot or drop hook of fn runs for the instantiation inst: the
// body made for it, or the generic one with inst's environment.
func (m *Machine) descCall(fn sem.EntityID, inst sem.TypeID, all []*obj) (*midir.Func, []*obj) {
	body, _ := m.lowered(fn)
	if b := m.p.Desc.Get(m.r, fn, inst); b != nil {
		body = b
	}
	if body.HasEnv {
		all = append(all, m.typeValue(m.envOf(fn, inst)))
	}
	if !instance.Bound(m.r, &m.p.Desc, fn) && len(m.r.Fn(fn).Witnesses) == 0 {
		return body, all
	}
	hidden, ok := instance.HiddenArgs(m.r, fn, inst)
	if !ok {
		m.abort("internal error: `" + m.r.Entity(fn).Name + "` is reached through a descriptor without its instantiation")
	}
	for range hidden.Witnesses {
		all = append(all, unitObj)
	}
	for _, a := range hidden.Consts {
		in := instance.ConstInst(a)
		all = append(all, m.immortal(m.constant(&in)))
	}
	return body, all
}

// envOf is the environment method fn runs with for a receiver of runtime type inst.
func (m *Machine) envOf(fn sem.EntityID, inst sem.TypeID) sem.TypeID {
	m.sh.derived.Lock()
	defer m.sh.derived.Unlock()
	key := envKey{fn, inst}
	if t, ok := m.sh.envs[key]; ok {
		return t
	}
	bound, _ := m.r.ReceiverArgs(fn, inst)
	tps := m.r.Fn(fn).TypeParams
	args := make([]sem.TypeID, len(tps))
	for i, p := range tps {
		args[i] = bound[p]
	}
	t := m.p.Canon.Of(m.r, instance.EnvOf(m.r, fn, args))
	m.sh.envs[key] = t
	return t
}

type envKey struct {
	fn   sem.EntityID
	inst sem.TypeID
}
