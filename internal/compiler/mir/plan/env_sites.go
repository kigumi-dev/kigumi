package plan

import (
	"kigumi/internal/compiler/mir/instance"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// siteTypes are the types f composes at runtime: those its boxes, exact tests, downcasts and records name, the
// environments of the bodies it calls or makes values of, and the free parameters of the lambdas it builds.
func (ps *envPass) siteTypes(f *midir.Func) []sem.TypeID {
	var out []sem.TypeID
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if t := ps.siteType(f, in); t != 0 {
				out = append(out, t)
			}
			if in.Op == midir.OpClosure && ps.p.R.Entity(in.Ent).Kind == sem.EntClosure {
				for p := range ps.free[ps.p.ByEnt[in.Ent]] {
					out = append(out, ps.p.R.Types.Param(p))
				}
			}
		}
	}
	return out
}

// siteType is the runtime type in composes, or the environment it passes; 0 when it needs neither.
func (ps *envPass) siteType(f *midir.Func, in midir.Inst) sem.TypeID {
	r := ps.p.R
	switch in.Op {
	case midir.OpIsType:
		if instance.TypeTest(r, in.Type) == instance.TestExact {
			return in.Type
		}
	case midir.OpBox, midir.OpLendBox:
		return in.Type
	case midir.OpRecord:
		if r.DropReadsInstance(in.Ent) {
			return f.Locals[in.Dst].Type
		}
	case midir.OpCall:
		if r.IsErrorAs(in.Ent) {
			opt, _ := r.Types.IsOption(f.Locals[in.Dst].Type)
			return opt
		}
	case midir.OpFnItem, midir.OpBind:
		if t, ok := errorAsValueTarget(r, f, in); ok {
			return t
		}
	}
	if _, env, ok := ps.calleeEnv(f, in); ok {
		return env
	}
	return 0
}

// errorAsValueTarget is the target of a std/error.as value: the payload of the Option its type returns.
func errorAsValueTarget(r *sem.Result, f *midir.Func, in midir.Inst) (sem.TypeID, bool) {
	if !r.IsErrorAs(in.Ent) || in.Op == midir.OpBind && len(in.Args) > 0 {
		return 0, false
	}
	n := r.Types.Node(f.Locals[in.Dst].Type)
	if n.Kind != sem.KFn {
		return 0, false
	}
	return r.Types.IsOption(n.Elem)
}

// calleeEnv is the body in calls or makes a value of, when that body takes an environment, and the environment
// it passes.
func (ps *envPass) calleeEnv(f *midir.Func, in midir.Inst) (sem.EntityID, sem.TypeID, bool) {
	r := ps.p.R
	var args []sem.TypeID
	ok := false
	switch in.Op {
	case midir.OpCall:
		if !ps.calleeTakesEnv(in.Ent) {
			return 0, 0, false
		}
		args, ok = ps.p.bindTypeArgsVia(f, in, ps.substOf(f), bindRuntime)
	case midir.OpFutureNew:
		item, found := soleDef(f, in.Args[0])
		if !found || item.Op != midir.OpFnItem || !ps.calleeTakesEnv(item.Ent) {
			return 0, 0, false
		}
		call := in
		call.Ent, call.Args = item.Ent, in.Args[1:]
		args, ok = ps.p.bindTypeArgsVia(f, call, ps.substOf(f), bindRuntime)
		in = call
	case midir.OpFnItem, midir.OpBind:
		if !ps.calleeTakesEnv(in.Ent) || in.Op == midir.OpFnItem && in.Async && ps.futureItem(f, in.Dst) {
			return 0, 0, false
		}
		if t, wit := ps.witnessTypes(f)[in.Dst]; wit {
			args, ok = ps.receiverArgs(in.Ent, t)
		} else {
			args, ok = ps.itemArgs(f, in)
		}
	case midir.OpClosure:
		if in.Form() != midir.FormMethod || !ps.calleeTakesEnv(in.Ent) {
			return 0, 0, false
		}
		args, ok = ps.p.valueArgs(f, in, ps.substOf(f))
	default:
		return 0, 0, false
	}
	if !ok {
		args = ps.selfArgs(in.Ent)
	}
	return in.Ent, ps.p.Canon.Of(r, instance.EnvOf(r, in.Ent, args)), true
}

// itemArgs binds the type arguments of a function value: the instantiation sem recorded for it, or its type.
func (ps *envPass) itemArgs(f *midir.Func, in midir.Inst) ([]sem.TypeID, bool) {
	if args, ok := ps.p.valueArgs(f, in, ps.substOf(f)); ok {
		return args, true
	}
	return ps.p.bindTypeArgsVia(f, in, ps.substOf(f), bindRuntime)
}

// selfArgs stand gen's parameters for themselves: a binding that failed leaves them for the environment the
// caller has, which is gen's own when gen calls itself.
func (ps *envPass) selfArgs(gen sem.EntityID) []sem.TypeID {
	r := ps.p.R
	tps := r.Fn(gen).TypeParams
	out := make([]sem.TypeID, len(tps))
	for i, p := range tps {
		out[i] = r.Types.Param(p)
	}
	return out
}

// futureItem reports whether fn item l only names the callee of a Future the body builds.
func (ps *envPass) futureItem(f *midir.Func, l midir.LocalID) bool {
	used := false
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			for k, a := range in.Args {
				if a != l {
					continue
				}
				if in.Op != midir.OpFutureNew || k != 0 {
					return false
				}
				used = true
			}
		}
		for _, a := range blk.Term.Args {
			if a == l {
				return false
			}
		}
	}
	return used
}

// substOf maps the types sem recorded for f's generic body to f's own: an instance substitutes its arguments.
func (ps *envPass) substOf(f *midir.Func) func(sem.TypeID) sem.TypeID {
	r := ps.p.R
	sub := r.ClosureSubst(f.Ent)
	if in, ok := r.InstanceOf(f.Ent); ok && len(in.Args) > 0 {
		sub = r.InstanceSubst(in.Gen, in.Args)
	}
	if sub == nil {
		return func(t sem.TypeID) sem.TypeID { return t }
	}
	return func(t sem.TypeID) sem.TypeID { return r.Types.Subst(t, sub) }
}

// soleDef finds the single instruction defining l.
func soleDef(f *midir.Func, l midir.LocalID) (midir.Inst, bool) {
	var found midir.Inst
	n := 0
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op.Yields() && in.Dst == l {
				found, n = in, n+1
			}
		}
	}
	return found, n == 1
}
