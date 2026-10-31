package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (p *callVerifier) declaredSig(ent sem.EntityID) (params []sem.TypeID, ret sem.TypeID, hidden int, variadic bool) {
	info := p.R.Fn(ent)
	sig := p.R.Types.Node(info.Sig)
	if info.Recv != sem.RecvNone {
		params = append(params, p.R.Entity(info.SelfParam).Type)
	}
	return append(params, sig.Args...), sig.Elem, len(info.Witnesses) + len(info.ConstParams), sig.Flags&sem.FnCVariadic != 0
}

func (p *callVerifier) declaredCallMismatch(f *midir.Func, in midir.Inst) string {
	params, ret, hidden, variadic := p.declaredSig(in.Ent)
	name := p.R.Entity(in.Ent).Name
	// A foreign symbol call is declared with only its receiver; its arguments and result come from a type argument.
	typed := p.R.Entity(in.Ent).Flags&sem.EfStd != 0 && StdKey(p.R, in.Ent, true) == "dl.Symbol.call"
	if len(in.Args) < len(params) || (len(in.Args) > len(params)+hidden && !variadic && !typed) {
		return "passes " + itoa(len(in.Args)) + " values to " + name + ", which takes " + itoa(len(params))
	}
	for k, want := range params {
		if !p.compatibleGeneric(want, f.Locals[in.Args[k]].Type) {
			return "passes " + p.R.TypeString(f.Locals[in.Args[k]].Type) + " as argument " + itoa(k) + " of " + name + ", which takes " + p.R.TypeString(want)
		}
	}
	if in.Op.Yields() && !typed && !p.compatibleGeneric(ret, f.Locals[in.Dst].Type) {
		return "takes " + p.R.TypeString(f.Locals[in.Dst].Type) + " from " + name + ", which returns " + p.R.TypeString(ret)
	}
	return ""
}

func (p *callVerifier) bindMismatch(f *midir.Func, in midir.Inst) string {
	callee := p.ByEnt[in.Ent]
	var params []sem.TypeID
	var ret sem.TypeID
	if callee != nil {
		if msg := p.captureMismatch(f, in, callee); msg != "" {
			return msg
		}
		for _, prm := range callee.Params {
			params = append(params, callee.Locals[prm].Type)
		}
		ret = callee.Ret
		params = params[:len(params)-len(in.Args)]
	} else {
		var hidden int
		params, ret, hidden, _ = p.declaredSig(in.Ent)
		if len(in.Args) > hidden {
			return "captures " + itoa(len(in.Args)) + " values for a function with " + itoa(hidden) + " hidden parameters"
		}
	}
	if info := p.R.Fn(in.Ent); info.Recv != sem.RecvNone && len(params) > 0 {
		params = params[1:]
	}
	dst := f.Locals[in.Dst].Type
	node := p.R.Types.Node(p.fnType(p.derefAll(dst)))
	if p.looseType(dst) || node.Kind != sem.KFn {
		if p.looseType(dst) {
			return ""
		}
		return "yields " + p.R.TypeString(dst) + ", not a function"
	}
	if len(node.Args) != len(params) {
		return "yields a function of " + itoa(len(node.Args)) + " parameters for " + p.R.Entity(in.Ent).Name + ", which leaves " + itoa(len(params))
	}
	for k, want := range params {
		if !p.compatibleGeneric(want, node.Args[k]) {
			return "yields " + p.R.TypeString(dst) + " for " + p.R.Entity(in.Ent).Name + ", whose parameter " + itoa(k) + " is " + p.R.TypeString(want)
		}
	}
	if !in.Async && !p.compatibleGeneric(ret, node.Elem) {
		return "yields " + p.R.TypeString(dst) + " for " + p.R.Entity(in.Ent).Name + ", which returns " + p.R.TypeString(ret)
	}
	return ""
}
