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

// directSig stays a function of the declaration alone, so a call site and
// the definition always agree on the signature.
func (p *Program) directSig(f *Func) *DirectSig {
	if f.Async || f == p.Entry || p.R.Entity(f.Ent).Kind != sem.EntFn {
		return nil
	}
	info := p.R.Fn(f.Ent)
	if info.Naked || info.Export != "" || info.Abi != "" {
		return nil
	}
	s := &DirectSig{Params: make([]sem.TypeID, len(f.Params))}
	any := false
	for i, prm := range f.Params {
		l := f.Locals[prm]
		if l.Cell || l.Borrowed {
			continue
		}
		if p.R.DirectAgg(l.Type) {
			s.Params[i], any = l.Type, true
		}
	}
	if t, ok := p.directRet(f); ok {
		s.Ret, any = t, true
	}
	if !any {
		return nil
	}
	return s
}

func (p *Program) directRet(f *Func) (sem.TypeID, bool) {
	var found sem.TypeID
	for _, blk := range f.Blocks {
		if blk.Term.Op != TermReturn {
			continue
		}
		t := f.Locals[blk.Term.Args[0]].Type
		if !p.R.DirectAgg(t) || (found != 0 && found != t) {
			return 0, false
		}
		found = t
	}
	return found, found != 0
}
