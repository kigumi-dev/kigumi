package verify

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (p *verifier) cCallMismatch(f *midir.Func, in midir.Inst) string {
	tt := p.R.Types
	if !p.looseType(in.Type) && !tt.IsCFn(in.Type) {
		return "calls through " + p.R.TypeString(in.Type) + ", which is no C function pointer"
	}
	if callee := f.Locals[in.Args[0]].Type; !p.compatible(in.Type, callee) {
		return "calls %" + f.Locals[in.Args[0]].Name + " of " + p.R.TypeString(callee) + " as " + p.R.TypeString(in.Type)
	}
	node := tt.Node(in.Type)
	if node.Kind != sem.KFn {
		return ""
	}
	args := in.Args[1:]
	if len(args) < len(node.Args) || len(args) > len(node.Args) && node.Flags&sem.FnCVariadic == 0 {
		return "passes " + itoa(len(args)) + " values to a C function of " + itoa(len(node.Args))
	}
	for k, want := range node.Args {
		if got := f.Locals[args[k]].Type; !p.compatible(want, got) {
			return "passes " + p.R.TypeString(got) + " as argument " + itoa(k) + " of " + p.R.TypeString(in.Type)
		}
	}
	if got := f.Locals[in.Dst].Type; in.Op.Yields() && !p.compatible(node.Elem, got) {
		return "takes " + p.R.TypeString(got) + " from a C function returning " + p.R.TypeString(node.Elem)
	}
	return ""
}
