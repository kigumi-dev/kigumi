package mir

type callVerifier struct {
	*verifier
	sole *soleIndex
}

func (p *verifier) verifyCalls(f *Func) *Error {
	c := &callVerifier{verifier: p, sole: newSoleIndex(f)}
	return c.check(f)
}
