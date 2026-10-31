package lsp

import (
	"strings"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// hoverText shows a declaration as written in its source, followed by its
// doc comment; entities without source fall back to describe.
func hoverText(res *sem.Result, ent sem.EntityID, at sem.TypeID) string {
	code, doc := describe(res, ent, at), ""
	if e := res.Entity(ent); e.File != 0 {
		t := res.Tree(e.File)
		if head := syntax.DeclHead(t, e.Node); head != "" {
			code = head
		}
		doc = syntax.DocText(t, e.Node)
	}
	out := "```kigumi\n" + code + "\n```"
	if doc != "" {
		out += "\n\n" + doc
	}
	if req := ctxRequiresText(res, ent); req != "" {
		out += "\n\n" + req
	}
	return out
}

// definedValueHover shows `.value`, a defined type's package-private
// representation projection, rather than ent's own declaration.
func definedValueHover(res *sem.Result, ent sem.EntityID, at sem.TypeID) string {
	name := res.Entity(ent).Name
	return "```kigumi\n" + name + ".value: " + res.TypeString(at) + "\n```\n\npackage-private representation of `" + name + "`"
}

// ctxRequiresText lists a function's inferred context requirement, as
// canonical `package.name` per key.
func ctxRequiresText(res *sem.Result, ent sem.EntityID) string {
	if res.Entity(ent).Kind != sem.EntFn {
		return ""
	}
	keys := res.Fn(ent).CtxRequires
	if len(keys) == 0 {
		return ""
	}
	names := make([]string, len(keys))
	for i, k := range keys {
		ke := res.Entity(k)
		names[i] = res.Packages[ke.Pkg].Path + "." + ke.Name
	}
	return "requires: " + strings.Join(names, ", ")
}

// describe renders an entity as `kind name: type`.
func describe(res *sem.Result, ent sem.EntityID, at sem.TypeID) string {
	e := res.Entity(ent)
	switch e.Kind {
	case sem.EntFn:
		name := e.Name
		if owner := res.Fn(ent).Owner; owner != 0 {
			name = res.Entity(owner).Name + "." + name
		}
		sig := res.TypeString(res.Fn(ent).Sig)
		i := strings.Index(sig, "fn")
		return sig[:i] + "fn " + name + sig[i+2:]
	case sem.EntType, sem.EntAlias, sem.EntInterface:
		return e.Kind.String() + " " + e.Name
	case sem.EntPackage, sem.EntImport:
		return "package " + e.Name
	case sem.EntVariant:
		return "variant " + res.Entity(e.Parent).Name + "." + e.Name
	case sem.EntField:
		return "field " + e.Name + ": " + res.TypeString(e.Type)
	}
	if at == 0 && e.Kind == sem.EntParam && e.Type != 0 {
		return e.Kind.String() + " " + e.Name + ": " + res.ParamTypeString(ent)
	}
	if at == 0 {
		at = e.Type
	}
	if at == 0 {
		return e.Kind.String() + " " + e.Name
	}
	return e.Kind.String() + " " + e.Name + ": " + res.TypeString(at)
}
