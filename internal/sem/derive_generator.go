package sem

// DeriveGenerator produces the source for one DeriveRequest; the default,
// legacyDeriveGenerator, keeps every caller but the real pipeline on today's behavior.
type DeriveGenerator func(r *Result, req DeriveRequest) (string, error)

// generatorFailureCode reads a generator error's Kind by method set, so sem
// never imports internal/schema (which imports sem) to get a diagnostic code.
func generatorFailureCode(err error) Code {
	kind, ok := err.(interface{ DiagKind() string })
	if !ok {
		return cDeriveGeneratorFailed
	}
	switch kind.DiagKind() {
	case "no-schema":
		return cDeriveGeneratorNoSchema
	case "unsupported":
		return cDeriveGeneratorUnsupported
	case "nondeterministic":
		return cDeriveGeneratorNondeterministic
	default:
		return cDeriveGeneratorFailed
	}
}

// legacyDeriveGenerator reproduces derive_json_text.go/derive_proto_text.go
// exactly; a throwaway deriver is safe since its methods only read g.r.
func legacyDeriveGenerator(r *Result, req DeriveRequest) (string, error) {
	g := &deriver{r: r}
	d := deriveRequest{decl: req.Decl, iface: req.Interface}
	info := r.typeDecl(d.decl)
	switch {
	case isProto(d.iface):
		return g.proto(d, info), nil
	case info.Form == FormAdt:
		return g.adt(d, info), nil
	default:
		return g.record(d, info), nil
	}
}
