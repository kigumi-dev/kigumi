package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// directSig reads sem's direct-carrier predicates, then excludes pin: a
// lent parameter needs an object only the caller can own across the call.
func (p *Program) directSig(f *midir.Func, pin []bool) *midir.DirectSig {
	if f.Async {
		return nil
	}
	if p.EnvABI(f) {
		return p.envSig(f, pin)
	}
	if !p.R.DirectCarried(f.Ent) {
		return nil
	}
	s := &midir.DirectSig{Params: make([]sem.TypeID, len(f.Params))}
	any := false
	for i, prm := range f.Params {
		t := f.Locals[prm].Type
		if !pin[prm] && p.R.DirectParamSlot(f.Ent, f.Locals[prm].Ent) && p.R.DirectAgg(t) {
			s.Params[i], any = t, true
		}
	}
	if t := p.R.DirectResultType(f.Ent, f.Ret); t != 0 {
		s.Ret, any = t, true
	}
	if !any {
		return nil
	}
	return s
}
