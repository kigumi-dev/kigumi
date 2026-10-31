package schema

import (
	"strings"

	"kigumi/internal/compiler/sem"
)

// GeneratedArtifact is one derive generator's record. File-level
// granularity: ContentHash is the batched file's hash, not per-declaration.
type GeneratedArtifact struct {
	Generator   string   `json:"generator"`
	Origin      *DeclRef `json:"origin,omitempty"`
	InputDigest string   `json:"inputDigest"`
	Package     string   `json:"package"`
	File        string   `json:"file"`
	ContentHash string   `json:"contentHash"`
}

// BuildProvenance falls back to digesting the origin's DeclRef alone when
// it is outside byEntity's scope (a std type Eq/Hash derived).
func BuildProvenance(res *sem.Result, byEntity map[sem.EntityID]*Declaration) []GeneratedArtifact {
	type key struct{ pkg, suffix string }
	files := map[key]sem.GeneratedSource{}
	for _, g := range res.GeneratedSources() {
		suffix := "json.derived.kg"
		switch {
		case strings.HasSuffix(g.Name, "proto.derived.kg"):
			suffix = "proto.derived.kg"
		case strings.HasSuffix(g.Name, "display.derived.kg"):
			suffix = "display.derived.kg"
		}
		files[key{g.Package, suffix}] = g
	}
	var out []GeneratedArtifact
	for _, req := range res.DeriveRequests() {
		origin := declRefOf(res, req.Decl)
		var inputDigest string
		if d, ok := byEntity[req.Decl]; ok {
			inputDigest = digest(d)
		} else if origin != nil {
			inputDigest = digest(origin)
		}
		pkg := res.Packages[res.PackageOf(res.Entity(req.Decl).File)].Path
		if pkg == "" {
			// A predeclared type (Option, Result, TupleN) has no package of its
			// own; derive_gen.go and build_one.go both file it under std/prelude.
			pkg = builtinPkg
		}
		suffix := "json.derived.kg"
		switch req.Interface {
		case "Eq", "Hash":
			suffix = "proto.derived.kg"
		case "Display":
			suffix = "display.derived.kg"
		}
		g, ok := files[key{pkg, suffix}]
		if !ok {
			continue
		}
		out = append(out, GeneratedArtifact{
			Generator:   "derive:" + req.Interface,
			Origin:      origin,
			InputDigest: inputDigest,
			Package:     pkg,
			File:        suffix,
			ContentHash: digestBytes([]byte(g.Text)),
		})
	}
	return out
}
