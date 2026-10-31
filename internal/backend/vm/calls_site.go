package vm

import (
	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
)

type valueSiteKey struct {
	ent   sem.EntityID
	typ   sem.TypeID
	bound int
}

type valueSite struct {
	inst *mir.Inst
	fn   *mir.Func
}

// valueSite is the call site a primitive reached through a function value sees,
// typed by the value (ffi.sizeOf and error.as read types off the site).
func (m *Machine) valueSite(fv *obj, bound int) (*mir.Inst, *mir.Func) {
	key := valueSiteKey{fv.ent, fv.dynType(), bound}
	if s, ok := m.valueSites[key]; ok {
		return s.inst, s.fn
	}
	fn := &mir.Func{}
	inst := &mir.Inst{Op: mir.OpCall, Ent: fv.ent}
	if n := m.r.Types.Node(fv.dynType()); fv.dynType() != 0 && n.Kind == sem.KFn {
		fn.Locals = append(fn.Locals, mir.Local{Type: n.Elem})
		for i := range bound {
			fn.Locals = append(fn.Locals, mir.Local{})
			inst.Args = append(inst.Args, mir.LocalID(i+1))
		}
		for _, a := range n.Args {
			fn.Locals = append(fn.Locals, mir.Local{Type: a})
			inst.Args = append(inst.Args, mir.LocalID(len(fn.Locals)-1))
		}
	}
	if m.valueSites == nil {
		m.valueSites = map[valueSiteKey]valueSite{}
	}
	m.valueSites[key] = valueSite{inst, fn}
	return inst, fn
}
