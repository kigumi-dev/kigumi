package mir

import "kigumi/internal/compiler/sem"

func (p *verifier) verifyEntry() *Error {
	f := p.Entry
	if f == nil {
		return nil
	}
	if len(f.Params) > 1 {
		return &Error{Func: f.Name, Rule: "entry", Msg: "the entry takes " + itoa(len(f.Params)) + " parameters, but is called with at most a Host"}
	}
	for _, prm := range f.Params {
		n := p.R.Types.Node(f.Locals[prm].Type)
		if n.Kind != sem.KNamed || p.R.Entity(n.Ent).Name != "Host" {
			return &Error{Func: f.Name, Rule: "entry", Msg: "the entry takes " + p.R.TypeString(f.Locals[prm].Type) + ", not a Host"}
		}
	}
	return nil
}
