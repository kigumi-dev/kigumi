package sem

// DeriveGenerator produces the source for one DeriveRequest; CheckWith's
// caller supplies one (the real pipeline passes internal/toolchain/contract/schema's GenerateDerive).
type DeriveGenerator func(r *Result, req DeriveRequest) (string, error)

// generatorFailureCode reads a generator error's Kind by method set, so sem
// never imports internal/toolchain/contract/schema (which imports sem) to get a diagnostic code.
func generatorFailureCode(err error) Code {
	kind, ok := err.(interface{ DiagKind() string })
	if !ok {
		return cDeriveGeneratorFailed
	}
	if kind.DiagKind() == "unsupported" {
		return cDeriveGeneratorUnsupported
	}
	return cDeriveGeneratorFailed
}

// noDeriveGenerator is Check's generator; a module with nothing to derive
// never calls it, one that does needs CheckWith with a real generator instead.
func noDeriveGenerator(*Result, DeriveRequest) (string, error) {
	return "", &noGeneratorError{}
}

type noGeneratorError struct{}

func (*noGeneratorError) Error() string    { return "no derive generator configured; call CheckWith" }
func (*noGeneratorError) DiagKind() string { return "unsupported" }
