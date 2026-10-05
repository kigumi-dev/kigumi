package schema

import (
	"fmt"
	"strings"
)

type encodeGenerator struct{}
type decodeGenerator struct{}

func (encodeGenerator) ID() GeneratorID { return GenEncode }
func (decodeGenerator) ID() GeneratorID { return GenDecode }

func (g encodeGenerator) Generate(decl *Declaration) (*GeneratedUnit, error) {
	return &GeneratedUnit{Generator: GenEncode, Origin: declRef(decl), CacheKey: cacheKey(GenEncode, decl), Text: jsonText(decl, true)}, nil
}

func (g decodeGenerator) Generate(decl *Declaration) (*GeneratedUnit, error) {
	return &GeneratedUnit{Generator: GenDecode, Origin: declRef(decl), CacheKey: cacheKey(GenDecode, decl), Text: jsonText(decl, false)}, nil
}

func jsonText(decl *Declaration, encode bool) string {
	iface := "Decode"
	if encode {
		iface = "Encode"
	}
	binder := genericBinder(decl, "json."+iface)
	if len(decl.Variants) != 0 {
		if encode {
			return jsonEncodeAdt(decl, binder)
		}
		return jsonDecodeAdt(decl, binder)
	}
	if encode {
		return jsonEncodeRecord(decl, binder)
	}
	return jsonDecodeRecord(decl, binder)
}

// jsonNameID and jsonOmitNoneID match std/json's name/omitNone by resolved
// SchemaID (as metadataOf does), never by dotted text an alias could change.
var (
	jsonNameID     = declID("std/json", string(KindFn), "name")
	jsonOmitNoneID = declID("std/json", string(KindFn), "omitNone")
)

func jsonKey(f *Member) string {
	for _, m := range f.Metadata {
		if m.SchemaID == jsonNameID && len(m.Args) == 1 {
			if s, ok := m.Args[0].(string); ok {
				return s
			}
		}
	}
	return f.Name
}

func jsonOmitNone(f *Member) bool {
	for _, m := range f.Metadata {
		if m.SchemaID == jsonOmitNoneID {
			return true
		}
	}
	return false
}

func jsonEncodeRecord(decl *Declaration, binder string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "fn %s.toJson(self) -> json.Value {\n    let mut f = json.Fields.new()\n", binder)
	for _, f := range decl.Fields {
		if jsonOmitNone(f) {
			fmt.Fprintf(&sb, "    if self.%s is Some(_) {\n        f.put(%q, self.%s.toJson())\n    }\n", f.Name, jsonKey(f), f.Name)
		} else {
			fmt.Fprintf(&sb, "    f.put(%q, self.%s.toJson())\n", jsonKey(f), f.Name)
		}
	}
	sb.WriteString("    f.finish()\n}\n")
	return sb.String()
}

func jsonDecodeRecord(decl *Declaration, binder string) string {
	var sb strings.Builder
	instance := genericInstance(decl)
	fmt.Fprintf(&sb, "fn %s.fromJson(value: json.Value) -> %s! {\n    let mut o = json.Record.expect(value, %q)?\n    Ok(%s {", binder, instance, decl.Name, decl.Name)
	for i, f := range decl.Fields {
		if i > 0 {
			sb.WriteString(",")
		}
		reader := "field"
		if f.Type != nil && f.Type.WellKnown == WKOption {
			reader = "optional"
		}
		fmt.Fprintf(&sb, " %s: o.%s(%q)?", f.Name, reader, jsonKey(f))
	}
	sb.WriteString(" })\n}\n")
	return sb.String()
}

func jsonEncodeAdt(decl *Declaration, binder string) string {
	var sb strings.Builder
	name := decl.Name
	fmt.Fprintf(&sb, "fn %s.toJson(self) -> json.Value {\n    match self {\n", binder)
	for _, v := range decl.Variants {
		n := len(v.Payload)
		if n == 0 {
			fmt.Fprintf(&sb, "        %s.%s -> json.tag(%q)\n", name, v.Name, v.Name)
			continue
		}
		var binds, encoded []string
		for i := range n {
			binds = append(binds, fmt.Sprintf("p%d", i))
			encoded = append(encoded, fmt.Sprintf("p%d.toJson()", i))
		}
		fmt.Fprintf(&sb, "        %s.%s(%s) -> json.tagged(%q, Array.of(%s))\n", name, v.Name, strings.Join(binds, ", "), v.Name, strings.Join(encoded, ", "))
	}
	sb.WriteString("    }\n}\n")
	return sb.String()
}

func jsonDecodeAdt(decl *Declaration, binder string) string {
	var sb strings.Builder
	name := decl.Name
	instance := genericInstance(decl)
	fmt.Fprintf(&sb, "fn %s.fromJson(value: json.Value) -> %s! {\n    let mut t = json.Tagged.expect(value, %q)?\n", binder, instance, name)
	for _, v := range decl.Variants {
		n := len(v.Payload)
		if n == 0 {
			fmt.Fprintf(&sb, "    if t.name == %q {\n        t.arity(0)?\n        return Ok(%s.%s)\n    }\n", v.Name, name, v.Name)
			continue
		}
		var reads []string
		for range n {
			reads = append(reads, "t.next()?")
		}
		fmt.Fprintf(&sb, "    if t.name == %q {\n        t.arity(%d)?\n        return Ok(%s.%s(%s))\n    }\n", v.Name, n, name, v.Name, strings.Join(reads, ", "))
	}
	sb.WriteString("    t.unknown()\n}\n")
	return sb.String()
}
