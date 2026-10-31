package ir

import "kigumi/internal/compiler/sem"

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

func (s *DirectSig) Result() sem.TypeID {
	if s == nil {
		return 0
	}
	return s.Ret
}
