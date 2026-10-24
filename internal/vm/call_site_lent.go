package vm

import (
	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

// callValueAs is callValue for a caller declaring each argument lent (a bit in lent) or handed over (in given): a lent
// one a parameter of fv consumes is retained first, a handed one a parameter only borrows is released after.
func (m *Machine) callValueAs(fv *obj, args []*obj, lent, given uint32) *obj {
	if lent == 0 && given == 0 {
		return m.callValue(fv, args)
	}
	borrows, known := m.calleeLent(deref(fv))
	for i := 0; known && i < len(args) && i < 31; i++ {
		if lent>>i&1 == 1 && !borrows(i) {
			retain(args[i])
		}
	}
	r := m.callValue(fv, args)
	for i := 0; known && i < len(args) && i < 31; i++ {
		if given>>i&1 == 1 && borrows(i) {
			m.release(args[i])
		}
	}
	return r
}

// callValueLent is how a primitive calls its callback: every argument is a borrow of what the primitive holds.
func (m *Machine) callValueLent(fv *obj, args []*obj) *obj {
	return m.callValueAs(fv, args, 1<<31-1, 0)
}

func (m *Machine) callSite(in *mir.Inst, args []*obj) *obj {
	return m.callValueAs(args[0], args[1:], in.LendArgs, in.GiveArgs)
}

// calleeLent says which call arguments the function a value calls only borrows; known is false when the value only
// forwards (an interface requirement) or runs later (an async value).
func (m *Machine) calleeLent(fv *obj) (borrows func(int) bool, known bool) {
	if fv.k != kClosure || fv.creq() != nil || fv.async {
		return nil, false
	}
	callee := fv.cfn()
	switch {
	case fv.cstd() != "":
		return func(i int) bool { return fv.clent()>>i&1 == 1 }, true
	case callee != nil && m.r.Entity(callee.Ent).Kind == sem.EntFn:
		off := 0
		if env := fv.cenv(); env != nil && !fv.bind {
			off = len(env.fields)
		}
		return func(i int) bool { return m.p.ParamLent(callee.Ent, off+i) }, true
	case callee != nil && m.r.Entity(callee.Ent).Kind == sem.EntClosure:
		params := m.r.Closure(callee.Ent).Params
		return func(i int) bool { return i < len(params) && m.r.Types.Kind(m.r.Entity(params[i]).Type) == sem.KRef }, true
	}
	return nil, false
}
