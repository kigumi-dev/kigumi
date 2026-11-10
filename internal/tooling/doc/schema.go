package doc

import (
	"encoding/json"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/toolchain/contract/schema"
	"kigumi/internal/toolchain/driver"
	"kigumi/internal/toolchain/version"
)

// Declaration is one top-level declaration with its metadata attributes:
// the typed, source-ordered declaration query, exposed to build steps and
// generators as JSON.
type Declaration struct {
	Package    string      `json:"package"`
	Kind       string      `json:"kind"`
	Name       string      `json:"name"`
	Signature  string      `json:"signature"`
	Doc        string      `json:"doc,omitempty"`
	Attributes []Attribute `json:"attributes"`
	// UnresolvedAttributes names metadata items written on the declaration
	// that the checker could not resolve to a `@schema` function (E254), so
	// `--allow-errors` output never looks like the declaration carries none.
	UnresolvedAttributes []string  `json:"unresolvedAttributes,omitempty"`
	Params               []Member  `json:"params,omitempty"`
	Fields               []Member  `json:"fields,omitempty"`
	Variants             []Variant `json:"variants,omitempty"`
}

// Variant is one constructor of an ADT with its named payload.
type Variant struct {
	Name    string   `json:"name"`
	Payload []Member `json:"payload"`
}

type Attribute struct {
	Name string `json:"name"`
	Args []any  `json:"args"`
}

type Member struct {
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	Attributes []Attribute `json:"attributes"`
	// UnresolvedAttributes mirrors Declaration.UnresolvedAttributes for a
	// parameter or field.
	UnresolvedAttributes []string `json:"unresolvedAttributes,omitempty"`
}

// Declarations lists the module's own declarations in source order; attr,
// when set, keeps only those carrying that attribute (`web.controller`).
func Declarations(m *driver.Module, res *sem.Result, attr string) []Declaration {
	out := []Declaration{}
	for _, path := range m.Order {
		p := m.Packages[path]
		if p.Std || p.Module != "" {
			continue
		}
		for _, t := range p.Files {
			for d := range t.List(t.Root).IDs() {
				if decl, ok := declaration(res, t, d); ok && (attr == "" || decl.hasAttribute(attr)) {
					decl.Package = path
					out = append(out, decl)
				}
			}
		}
	}
	return out
}

// hasAttribute keeps a `--attr` filter matching an unresolved item too: the
// declaration is precisely what a generator asking for that attribute wants
// to see when it cannot be resolved.
func (d Declaration) hasAttribute(name string) bool {
	if hasAttr(d.Attributes, name) {
		return true
	}
	for _, n := range d.UnresolvedAttributes {
		if n == name {
			return true
		}
	}
	return false
}

func hasAttr(attrs []Attribute, name string) bool {
	for _, a := range attrs {
		if a.Name == name {
			return true
		}
	}
	return false
}

func declaration(res *sem.Result, t *syntax.Tree, d syntax.NodeID) (Declaration, bool) {
	ent := res.File(t).Defs[d]
	if ent == 0 {
		return Declaration{}, false
	}
	e := res.Entity(ent)
	decl := Declaration{Name: e.Name, Signature: syntax.DeclHead(t, d), Doc: syntax.DocText(t, d), Attributes: []Attribute{}}
	switch e.Kind {
	case sem.EntFn:
		info := res.Fn(ent)
		decl.Kind = "fn"
		if info.Owner != 0 {
			decl.Kind = "method"
			decl.Name = res.Entity(info.Owner).Name + "." + e.Name
		}
		decl.Attributes, decl.UnresolvedAttributes = attributes(t, info.Metadata)
		for _, p := range info.Params {
			pe := res.Entity(p)
			attrs, unresolved := attributes(t, res.Local(p).Metadata)
			decl.Params = append(decl.Params, Member{Name: pe.Name, Type: res.ParamTypeString(p), Attributes: attrs, UnresolvedAttributes: unresolved})
		}
	case sem.EntType:
		info := res.TypeDecl(ent)
		decl.Kind = "type"
		decl.Attributes, decl.UnresolvedAttributes = attributes(t, info.Metadata)
		for _, f := range info.Fields {
			fe := res.Entity(f)
			attrs, unresolved := attributes(t, res.Field(f).Metadata)
			decl.Fields = append(decl.Fields, Member{Name: fe.Name, Type: res.TypeString(fe.Type), Attributes: attrs, UnresolvedAttributes: unresolved})
		}
		for _, v := range info.Variants {
			vi := res.Variant(v)
			var payload []Member
			for i, pt := range vi.Payload {
				name := ""
				if i < len(vi.Names) {
					name = vi.Names[i]
				}
				payload = append(payload, Member{Name: name, Type: res.TypeString(pt), Attributes: []Attribute{}})
			}
			decl.Variants = append(decl.Variants, Variant{Name: res.Entity(v).Name, Payload: payload})
		}
	case sem.EntConst:
		decl.Kind = "const"
		decl.Attributes, decl.UnresolvedAttributes = attributes(t, res.Const(ent).Metadata)
	case sem.EntStatic:
		decl.Kind = "static"
		if res.Static(ent).Mut {
			decl.Kind = "static mut"
		}
		decl.Attributes, decl.UnresolvedAttributes = attributes(t, res.Static(ent).Metadata)
	case sem.EntInterface:
		decl.Kind = "interface"
	default:
		return Declaration{}, false
	}
	return decl, true
}

// attributes splits metadata refs into the resolved ones and the names of
// the ones the checker could not attribute to a `@schema` function. The
// unresolved names stay visible so `--allow-errors` output never reads as
// "this declaration has no metadata" while its signature still shows `@...`.
func attributes(t *syntax.Tree, refs []sem.MetaRef) ([]Attribute, []string) {
	out := []Attribute{}
	var unresolved []string
	for _, m := range refs {
		name := t.View(m.Node).AsMetaItem().Path().Join(".")
		if m.Unresolved {
			unresolved = append(unresolved, name)
			continue
		}
		a := Attribute{Name: name, Args: []any{}}
		for _, lit := range m.Args {
			a.Args = append(a.Args, literalJSON(lit))
		}
		out = append(out, a)
	}
	return out, unresolved
}

func literalJSON(lit sem.Literal) any {
	switch lit.Kind {
	case sem.LitInt:
		if lit.Int.IsInt64() {
			return lit.Int.Int64()
		}
		return lit.Int.String()
	case sem.LitFloat:
		f, _ := lit.Float.Float64()
		return f
	case sem.LitBool:
		return lit.Bool
	case sem.LitChar:
		return string(lit.Char)
	}
	return lit.Str
}

// SchemaIRJSON renders m's versioned typed Schema IR, stamping the given
// target/profile onto the envelope; SchemaJSON's legacy strings project from it.
func SchemaIRJSON(m *driver.Module, res *sem.Result, attr, target, profile string) ([]byte, error) {
	mod, _ := schema.Build(m, res, schema.Options{Attr: attr, Target: target, Profile: profile})
	return schema.ProjectJSON(mod)
}

// SchemaJSON renders the declarations for tools.
func SchemaJSON(decls []Declaration) string {
	wrapped := struct {
		FormatVersion int                `json:"formatVersion"`
		Toolchain     version.Provenance `json:"toolchain"`
		Declarations  []Declaration      `json:"declarations"`
	}{FormatVersion: version.SchemaFormatVersion, Toolchain: version.Current(), Declarations: decls}
	out, _ := json.MarshalIndent(wrapped, "", "  ")
	return string(out) + "\n"
}
