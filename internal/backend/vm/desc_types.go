package vm

import (
	"kigumi/internal/compiler/mir/instance"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// typeValue is the Int a runtime type travels as, the TypeID of its identity: one immortal per type, not counted
// as allocated, so passing an environment allocates nothing.
func (m *Machine) typeValue(t sem.TypeID) *obj {
	nk := int16(m.numKind(sem.TyI64))
	m.sh.derived.Lock()
	defer m.sh.derived.Unlock()
	if v, ok := m.sh.typeValues[t]; ok {
		return v
	}
	v := &obj{k: kInt, rc: -1, i: int64(t), numk: nk}
	m.sh.typeValues[t] = v
	return v
}

// immortal is a fresh value's uncounted immortal copy.
func (m *Machine) immortal(v *obj) *obj {
	c := new(obj)
	*c = *v
	c.rc = -1
	m.release(v)
	return c
}

func (m *Machine) typeBuiltin(in *midir.Inst, args []*obj) *obj {
	switch in.Str {
	case "type.param":
		return m.typeValue(m.r.Types.Node(sem.TypeID(deref(args[0]).i)).Args[in.Index])
	case "type.arg", "type.const":
		v := deref(args[0])
		if v.k != kBox || m.typeEnt(v.dynType()) != in.Ent {
			m.abort("internal error: a type test opened a box of another type")
		}
		arg := m.r.Types.Node(v.dynType()).Args[in.Index]
		if in.Str == "type.arg" {
			return m.typeValue(arg)
		}
		return m.constValue(arg)
	}
	return m.typeOf(in, args)
}

// typeOf fills in.Type's holes with the runtime types args carries, or for an opened const parameter its value.
func (m *Machine) typeOf(in *midir.Inst, args []*obj) *obj {
	sub := make(map[sem.EntityID]sem.TypeID, len(in.Holes))
	for i, p := range in.Holes {
		a := deref(args[i])
		if m.r.Types.IsRtType(m.siteFn.Locals[in.Args[i]].Type) {
			sub[p] = sem.TypeID(a.i)
		} else {
			sub[p] = m.constType(a, m.r.TypeParam(p).ConstType)
		}
	}
	m.sh.derived.Lock()
	t := m.r.Types.Subst(in.Type, sub)
	m.sh.derived.Unlock()
	return m.typeValue(t)
}

// constType is the type a const argument's value spells, at its full width.
func (m *Machine) constType(a *obj, ty sem.TypeID) sem.TypeID {
	m.sh.derived.Lock()
	defer m.sh.derived.Unlock()
	if a.k == kBool {
		return m.r.Types.ConstVal(int64(boolBit(a.b)), ty)
	}
	return m.r.Types.ConstValBig(bigOfWide(widen(a), isSigned(a.nkind())), ty)
}

// constValue is the value of const argument t, one immortal per argument.
func (m *Machine) constValue(t sem.TypeID) *obj {
	m.sh.derived.Lock()
	v, ok := m.sh.constVals[t]
	m.sh.derived.Unlock()
	if ok {
		return v
	}
	val, ty := m.r.Types.ConstValueBig(t)
	in := instance.ConstInst(sem.ConstArg{Kind: sem.ConstArgLit, Value: val, Type: ty})
	v = m.immortal(m.constant(&in))
	m.sh.derived.Lock()
	m.sh.constVals[t] = v
	m.sh.derived.Unlock()
	return v
}

func boolBit(b bool) int {
	if b {
		return 1
	}
	return 0
}
