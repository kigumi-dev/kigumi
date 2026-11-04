package verify

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (p *callVerifier) boundMismatch(f *midir.Func, in midir.Inst) string {
	if in.Ent == 0 || int(in.Ent) >= len(p.R.Entities) || p.R.Entity(in.Ent).Kind != sem.EntFn {
		return ""
	}
	info := p.R.Fn(in.Ent)
	if len(info.Witnesses) == 0 {
		return ""
	}
	params, _, _, _ := p.declaredSig(in.Ent)
	tt := p.R.Types
	for k, slot := range info.Witnesses {
		wi := len(params) + k
		if wi >= len(in.Args) {
			break
		}
		for j, want := range params {
			if j >= len(in.Args) {
				break
			}
			node := tt.Node(p.DerefAll(want))
			if node.Kind != sem.KParam || node.Ent != slot.Param {
				continue
			}
			if msg := p.boundArg(f, in.Args[wi], in.Args[j]); msg != "" {
				return "passes %" + f.Locals[in.Args[j]].Name + " as argument " + itoa(j) + " with a witness " + msg
			}
		}
	}
	return ""
}

func (p *callVerifier) boundArg(f *midir.Func, witness, val midir.LocalID) string {
	tt := p.R.Types
	got := p.DerefAll(f.Locals[val].Type)
	if p.looseType(got) {
		return ""
	}
	if name, ok := p.witnessParam(f, witness, 0); ok {
		if tt.Kind(got) == sem.KParam && p.R.Entity(tt.Node(got).Ent).Name == name || !p.mentionsParam(f, name) {
			return ""
		}
		return "for " + name + ", not of type " + p.R.TypeString(got)
	}
	def := p.witnessDef(f, witness)
	if def == nil || def.Op != midir.OpBind || int(def.Ent) >= len(p.R.Entities) || p.R.Entity(def.Ent).Kind != sem.EntFn {
		return ""
	}
	info := p.R.Fn(def.Ent)
	if info.Recv == sem.RecvNone {
		return ""
	}
	self := p.R.Entity(info.SelfParam).Type
	if !p.compatibleGeneric(self, got) {
		return "made for " + p.R.TypeString(self) + ", not " + p.R.TypeString(got)
	}
	return ""
}

func (p *callVerifier) witnessDef(f *midir.Func, l midir.LocalID) *midir.Inst {
	for range 5 {
		def := p.soleDefOf(f, l)
		if def == nil {
			return nil
		}
		switch def.Op {
		case midir.OpAlias, midir.OpCopy, midir.OpShare, midir.OpMove:
			l = def.Args[0]
		default:
			return def
		}
	}
	return nil
}

func (p *callVerifier) closureSigMismatch(f *midir.Func, in midir.Inst, callee *midir.Func) string {
	dst := f.Locals[in.Dst].Type
	node := p.R.Types.Node(p.FnType(p.DerefAll(dst)))
	if p.looseType(dst) || node.Kind != sem.KFn {
		return ""
	}
	if len(node.Args) != len(callee.Params)-len(in.Args) {
		return "yields a function of " + itoa(len(node.Args)) + " parameters for " + callee.Name + ", which leaves " + itoa(len(callee.Params)-len(in.Args))
	}
	for k := range node.Args {
		prm := callee.Params[midir.ArgParam(midir.BoundTrails(in.Op), len(in.Args), k)]
		if want := callee.Locals[prm].Type; !p.compatibleGeneric(want, node.Args[k]) {
			return "yields " + p.R.TypeString(dst) + " for " + callee.Name + ", whose parameter " + itoa(k) + " is " + p.R.TypeString(want)
		}
	}
	if node.Flags&uint16(sem.EffAsync) == 0 && !in.Async && !p.compatibleGeneric(callee.Ret, node.Elem) {
		return "yields " + p.R.TypeString(dst) + " for " + callee.Name + ", which returns " + p.R.TypeString(callee.Ret)
	}
	return ""
}

func (p *callVerifier) cellArgMismatch(f *midir.Func, in midir.Inst, callee *midir.Func, k int, arg midir.LocalID) string {
	if !callee.Locals[callee.Params[k]].Cell || p.mutReceiver(in) == k || p.canBeCell(f, arg, map[midir.LocalID]bool{}) {
		return ""
	}
	return "passes %" + f.Locals[arg].Name + ", which is no cell, as argument " + itoa(k) + " of " + callee.Name + ", which takes a cell"
}
