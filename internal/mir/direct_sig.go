package mir

import "kigumi/internal/sem"

// DirectSig names the parameters and the result a function passes as an
// unboxed aggregate; a zero entry means that position crosses as a carrier.
type DirectSig struct {
	Params []sem.TypeID
	Ret    sem.TypeID
}

func (s *DirectSig) Param(i int) sem.TypeID {
	if s == nil || i >= len(s.Params) {
		return 0
	}
	return s.Params[i]
}

func (s *DirectSig) result() sem.TypeID {
	if s == nil {
		return 0
	}
	return s.Ret
}

// directSig reads sem's direct-carrier predicates, then excludes pin: a
// lent parameter needs an object only the caller can own across the call.
func (p *Program) directSig(f *Func, pin []bool) *DirectSig {
	if f.Async || !p.R.DirectCarried(f.Ent) {
		return nil
	}
	s := &DirectSig{Params: make([]sem.TypeID, len(f.Params))}
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
