package verify

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (p *verifier) asmMismatch(f *midir.Func, in midir.Inst) string {
	info := p.R.AsmOf(f.File, in.Node)
	if info == nil {
		return ""
	}
	var inputs []sem.AsmOperand
	for _, op := range info.Operands {
		if op.Kind == sem.AsmIn || op.Kind == sem.AsmInOut {
			inputs = append(inputs, op)
		}
	}
	if len(in.Args) != len(inputs) {
		return "passes " + itoa(len(in.Args)) + " values to a block of " + itoa(len(inputs)) + " inputs"
	}
	for k, op := range inputs {
		if got := f.Locals[in.Args[k]].Type; !p.compatible(op.Type, got) {
			return "passes " + p.R.TypeString(got) + " as input " + itoa(k) + ", which is " + p.R.TypeString(op.Type)
		}
	}
	if got := f.Locals[in.Dst].Type; !p.compatible(info.Result, got) {
		return "yields " + p.R.TypeString(got) + " for a block that returns " + p.R.TypeString(info.Result)
	}
	if p.R.Types.Kind(info.Result) == sem.KNamed {
		fields := p.R.AggFields(info.Result)
		for _, op := range info.Operands {
			if op.Field >= len(fields) {
				return "fills field " + itoa(op.Field) + " of " + p.R.TypeString(info.Result) + ", which has " + itoa(len(fields))
			}
		}
	}
	return ""
}
