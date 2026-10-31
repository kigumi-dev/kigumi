package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (p *callVerifier) check(f *midir.Func) *Error {
	for bid, blk := range f.Blocks {
		bid := midir.BlockID(bid)
		for i, in := range blk.Insts {
			if msg := p.callMismatch(f, in); msg != "" {
				return fail("calls", bid, i, "%s %s", in.Op, msg)
			}
		}
	}
	return nil
}

func (p *callVerifier) callMismatch(f *midir.Func, in midir.Inst) string {
	switch in.Op {
	case midir.OpCall:
		var msg string
		if callee := p.ByEnt[in.Ent]; callee != nil {
			msg = p.bodyCallMismatch(f, in, callee)
		} else {
			msg = p.declaredCallMismatch(f, in)
		}
		if msg == "" {
			msg = p.boundMismatch(f, in)
		}
		return msg
	case midir.OpCallValue:
		return p.valueCallMismatch(f, in)
	case midir.OpCallC:
		return p.cCallMismatch(f, in)
	case midir.OpAsm:
		return p.asmMismatch(f, in)
	case midir.OpClosure:
		if callee := p.ByEnt[in.Ent]; callee != nil && in.Str != "applied" {
			if msg := p.captureMismatch(f, in, callee); msg != "" {
				return msg
			}
			return p.closureSigMismatch(f, in, callee)
		}
	case midir.OpBind:
		return p.bindMismatch(f, in)
	case midir.OpFutureNew:
		return p.futureMismatch(f, in)
	}
	return ""
}

func (p *callVerifier) futureMismatch(f *midir.Func, in midir.Inst) string {
	item := p.soleDefOf(f, in.Args[0])
	if item == nil || item.Op != midir.OpFnItem || !item.Async {
		return "runs %" + f.Locals[in.Args[0]].Name + ", which is not an async function item"
	}
	tt := p.R.Types
	dst := f.Locals[in.Dst].Type
	node := tt.Node(dst)
	if !p.looseType(dst) && (node.Kind != sem.KNamed || node.Ent != tt.FutureEnt() || len(node.Args) != 1) {
		return "yields " + p.R.TypeString(dst) + ", not a Future"
	}
	callee := p.ByEnt[item.Ent]
	if callee == nil {
		return ""
	}
	if len(in.Args)-1 != len(callee.Params) {
		return "passes " + itoa(len(in.Args)-1) + " values to " + callee.Name + ", which takes " + itoa(len(callee.Params))
	}
	for k, a := range in.Args[1:] {
		if want := callee.Locals[callee.Params[k]].Type; !p.compatibleGeneric(want, f.Locals[a].Type) {
			return "passes " + p.R.TypeString(f.Locals[a].Type) + " as argument " + itoa(k) + " of " + callee.Name + ", which takes " + p.R.TypeString(want)
		}
		if msg := p.cellArgMismatch(f, in, callee, k, a); msg != "" {
			return msg
		}
	}
	if node.Kind == sem.KNamed && len(node.Args) == 1 && !p.compatibleGeneric(callee.Ret, node.Args[0]) {
		return "yields " + p.R.TypeString(dst) + " for " + callee.Name + ", which returns " + p.R.TypeString(callee.Ret)
	}
	return ""
}

func soleDef(f *midir.Func, l midir.LocalID) *midir.Inst {
	var def *midir.Inst
	for _, blk := range f.Blocks {
		for k := range blk.Insts {
			if d := &blk.Insts[k]; d.Op.Yields() && d.Dst == l {
				if def != nil {
					return nil
				}
				def = d
			}
		}
	}
	return def
}

func (p *callVerifier) captureMismatch(f *midir.Func, in midir.Inst, callee *midir.Func) string {
	if len(in.Args) > len(callee.Params) {
		return "captures " + itoa(len(in.Args)) + " values for a body of " + itoa(len(callee.Params)) + " parameters"
	}
	if in.Op == midir.OpClosure && p.R.Entity(in.Ent).Kind == sem.EntClosure {
		if n := len(p.R.Closure(in.Ent).Captures); n != len(in.Args) {
			return "captures " + itoa(len(in.Args)) + " values for a closure of " + itoa(n) + " captures"
		}
	}
	first := 0
	if in.Op == midir.OpBind {
		first = len(callee.Params) - len(in.Args)
	}
	open := in.Op == midir.OpBind || in.Str == "method"
	for k, a := range in.Args {
		if want := callee.Locals[callee.Params[first+k]].Type; !p.compat(want, f.Locals[a].Type, open) {
			return "captures " + p.R.TypeString(f.Locals[a].Type) + " for a parameter of " + p.R.TypeString(want) + " of " + callee.Name
		}
	}
	return ""
}

func (p *callVerifier) bodyCallMismatch(f *midir.Func, in midir.Inst, callee *midir.Func) string {
	if len(in.Args) != len(callee.Params) {
		return "passes " + itoa(len(in.Args)) + " values to " + callee.Name + ", which takes " + itoa(len(callee.Params))
	}
	for k, a := range in.Args {
		if want := callee.Locals[callee.Params[k]].Type; !p.compatibleGeneric(want, f.Locals[a].Type) {
			return "passes " + p.R.TypeString(f.Locals[a].Type) + " as argument " + itoa(k) + " of " + callee.Name + ", which takes " + p.R.TypeString(want)
		}
		if msg := p.cellArgMismatch(f, in, callee, k, a); msg != "" {
			return msg
		}
	}
	if in.Op.Yields() && !p.compatibleGeneric(callee.Ret, f.Locals[in.Dst].Type) {
		return "takes " + p.R.TypeString(f.Locals[in.Dst].Type) + " from " + callee.Name + ", which returns " + p.R.TypeString(callee.Ret)
	}
	return ""
}

func (p *callVerifier) valueCallMismatch(f *midir.Func, in midir.Inst) string {
	args := in.Args[1:]
	if in.Ent != 0 {
		info := p.R.Fn(in.Ent)
		want := len(p.R.Types.Node(info.Sig).Args)
		if info.Recv != sem.RecvNone {
			want++
		}
		if len(args) != want {
			return "passes " + itoa(len(args)) + " values to the witness of " + p.R.Entity(in.Ent).Name + ", which takes " + itoa(want)
		}
		return p.witnessCallMismatch(f, in, info.Recv != sem.RecvNone)
	}
	t := p.fnType(p.derefAll(f.Locals[in.Args[0]].Type))
	node := p.R.Types.Node(t)
	if node.Kind != sem.KFn {
		return ""
	}
	if len(args) != len(node.Args) && node.Flags&sem.FnCVariadic == 0 {
		return "passes " + itoa(len(args)) + " values to a function of " + itoa(len(node.Args))
	}
	for k, a := range args {
		if k < len(node.Args) && !p.compatible(node.Args[k], f.Locals[a].Type) {
			return "passes " + p.R.TypeString(f.Locals[a].Type) + " as argument " + itoa(k) + " of " + p.R.TypeString(t)
		}
	}
	if in.Op.Yields() && node.Flags&uint16(sem.EffAsync) == 0 && !p.compatible(node.Elem, f.Locals[in.Dst].Type) {
		return "takes " + p.R.TypeString(f.Locals[in.Dst].Type) + " from a function of type " + p.R.TypeString(t)
	}
	return ""
}
