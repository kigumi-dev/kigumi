package instance

import "kigumi/internal/compiler/sem"

// Enclosing is the function whose type parameters a body of ent sees: ent itself, or the function a closure is
// written in.
func Enclosing(r *sem.Result, ent sem.EntityID) sem.EntityID {
	for ent != 0 {
		switch r.Entity(ent).Kind {
		case sem.EntFn:
			return ent
		case sem.EntClosure:
			ent = r.Entity(ent).Parent
		default:
			return 0
		}
	}
	return 0
}

// Scope lists the type and const parameters fn's body binds, own then the receiver's copies, lifetimes left out:
// what its runtime environment carries.
func Scope(r *sem.Result, fn sem.EntityID) []sem.EntityID {
	if fn == 0 || r.Entity(fn).Kind != sem.EntFn {
		return nil
	}
	if _, minted := r.InstanceOf(fn); minted {
		return nil
	}
	info := r.Fn(fn)
	var out []sem.EntityID
	seen := map[sem.EntityID]bool{}
	for _, p := range append(append([]sem.EntityID(nil), info.TypeParams...), info.RecvParams...) {
		if !seen[p] && !r.IsLifetimeParam(p) {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

// EnvType is the type fn's environment stands for: a plain fn's one parameter itself, the receiver when it spells
// every parameter (a box's runtime type is then its methods' environment), else fn applied to its parameters.
func EnvType(r *sem.Result, fn sem.EntityID) (sem.TypeID, bool) {
	scope := Scope(r, fn)
	switch {
	case len(scope) == 0:
		return 0, false
	case len(scope) == 1 && r.Fn(fn).Recv == sem.RecvNone:
		return r.Types.Param(scope[0]), true
	}
	if self, ok := receiverEnv(r, fn, scope); ok {
		return self, true
	}
	args := make([]sem.TypeID, len(scope))
	for i, p := range scope {
		args[i] = r.Types.Param(p)
	}
	return r.Types.Named(fn, args), true
}

func receiverEnv(r *sem.Result, fn sem.EntityID, scope []sem.EntityID) (sem.TypeID, bool) {
	info := r.Fn(fn)
	if info.Recv == sem.RecvNone || info.SelfParam == 0 {
		return 0, false
	}
	self := r.Entity(info.SelfParam).Type
	if r.Types.Kind(self) == sem.KRef {
		self = r.Types.Node(self).Elem
	}
	n := r.Types.Node(self)
	if n.Kind != sem.KNamed {
		return 0, false
	}
	at := map[sem.EntityID]bool{}
	for _, a := range n.Args {
		an := r.Types.Node(a)
		if an.Kind != sem.KParam || at[an.Ent] {
			return 0, false
		}
		at[an.Ent] = true
	}
	for _, p := range scope {
		if !at[p] {
			return 0, false
		}
	}
	return self, true
}

// WholeEnv is EnvIndex's position of the parameter an environment is itself.
const WholeEnv = -1

// EnvIndex is where each parameter of env, an EnvType, sits among its arguments.
func EnvIndex(r *sem.Result, env sem.TypeID) map[sem.EntityID]int {
	if n := r.Types.Node(env); n.Kind == sem.KParam {
		return map[sem.EntityID]int{n.Ent: WholeEnv}
	}
	out := map[sem.EntityID]int{}
	for i, a := range r.Types.Node(env).Args {
		if n := r.Types.Node(a); n.Kind == sem.KParam {
			out[n.Ent] = i
		}
	}
	return out
}

// EnvOf is the environment a call of fn passes when its type parameters are args, in TypeParams order.
func EnvOf(r *sem.Result, fn sem.EntityID, args []sem.TypeID) sem.TypeID {
	env, _ := EnvType(r, fn)
	return r.Types.Subst(env, r.InstanceSubst(fn, args))
}
