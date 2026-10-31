package sem

// DeriveRequest is one (type, interface) pair the checker generated a
// method for, exported for provenance tooling (internal/toolchain/contract/schema).
type DeriveRequest struct {
	Decl      EntityID
	Interface string
}

// DeriveRequests lists every derive request the check settled, in request
// order.
func (r *Result) DeriveRequests() []DeriveRequest {
	out := make([]DeriveRequest, len(r.derives))
	for i, d := range r.derives {
		out[i] = DeriveRequest{Decl: d.decl, Interface: d.iface}
	}
	return out
}

// GeneratedSource is one generated Kigumi file Check added to the module,
// exported for provenance tooling.
type GeneratedSource struct {
	Package string
	Name    string
	Text    string
}

// GeneratedSources lists every file Check generated, in generation order.
func (r *Result) GeneratedSources() []GeneratedSource {
	out := make([]GeneratedSource, len(r.generated))
	for i, g := range r.generated {
		out[i] = GeneratedSource{Package: g.path, Name: g.name, Text: g.text}
	}
	return out
}
