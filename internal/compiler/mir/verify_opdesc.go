package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
)

// opDescMismatch holds a std primitive call to its registry descriptor: the
// classes the runtime reads for each argument and the result.
func (p *verifier) opDescMismatch(f *midir.Func, in midir.Inst) string {
	id, ok := in.StdOp()
	if !ok {
		return ""
	}
	desc := opdef.Registry.Ops[id].Desc
	if len(in.Args) < len(desc.Args) {
		return "the runtime op " + opdef.StdOpNames[id] + " takes " + itoa(len(desc.Args)) + " arguments, got " + itoa(len(in.Args))
	}
	for k, want := range desc.Args {
		if msg := p.opClassMismatch(want, f.Locals[in.Args[k]].Type); msg != "" {
			return "argument " + itoa(k) + " " + msg + " (op " + opdef.StdOpNames[id] + ")"
		}
	}
	if in.Op.Yields() {
		if msg := p.opClassMismatch(desc.Result, f.Locals[in.Dst].Type); msg != "" {
			return "result " + msg + " (op " + opdef.StdOpNames[id] + ")"
		}
	}
	return ""
}
