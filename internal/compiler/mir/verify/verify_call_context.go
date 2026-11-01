package verify

import midir "kigumi/internal/compiler/mir/ir"

type callVerifier struct {
	*verifier
	sole *soleIndex
}

func (p *verifier) verifyCalls(f *midir.Func) *Error {
	c := &callVerifier{verifier: p, sole: newSoleIndex(f)}
	return c.check(f)
}
