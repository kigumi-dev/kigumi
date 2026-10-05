package schema

import (
	"fmt"
	"strings"
)

type eqGenerator struct{}
type hashGenerator struct{}

func (eqGenerator) ID() GeneratorID   { return GenEq }
func (hashGenerator) ID() GeneratorID { return GenHash }

func (g eqGenerator) Generate(decl *Declaration) (*GeneratedUnit, error) {
	return &GeneratedUnit{Generator: GenEq, Origin: declRef(decl), CacheKey: cacheKey(GenEq, decl), Text: equalsText(decl)}, nil
}

func (g hashGenerator) Generate(decl *Declaration) (*GeneratedUnit, error) {
	return &GeneratedUnit{Generator: GenHash, Origin: declRef(decl), CacheKey: cacheKey(GenHash, decl), Text: hashText(decl)}, nil
}

// protoVis mirrors the type's own visibility, so the generated method is
// usable from other packages.
func protoVis(decl *Declaration) string {
	if decl.Visibility == VisPub {
		return "pub "
	}
	return ""
}

func genericBinder(decl *Declaration, iface string) string {
	if len(decl.Params) == 0 {
		return decl.Name
	}
	var ps []string
	for _, p := range decl.Params {
		if p.Const {
			ps = append(ps, p.Name)
			continue
		}
		ps = append(ps, p.Name+": "+iface)
	}
	return decl.Name + "[" + strings.Join(ps, ", ") + "]"
}

func genericInstance(decl *Declaration) string {
	if len(decl.Params) == 0 {
		return decl.Name
	}
	var ps []string
	for _, p := range decl.Params {
		ps = append(ps, p.Name)
	}
	return decl.Name + "[" + strings.Join(ps, ", ") + "]"
}

func equalsText(decl *Declaration) string {
	var sb strings.Builder
	name := decl.Name
	fmt.Fprintf(&sb, "%spure fn %s.equals(self, other: &%s) -> Bool {\n", protoVis(decl), genericBinder(decl, "Eq"), genericInstance(decl))
	if len(decl.Variants) == 0 {
		var terms []string
		for _, f := range decl.Fields {
			// Going through the protocol (not `==`) gives a Float field total Eq
			// rather than raw IEEE `==`; field access is never itself a reference.
			terms = append(terms, fmt.Sprintf("self.%s.equals(&other.%s)", f.Name, f.Name))
		}
		if len(terms) == 0 {
			terms = []string{"true"}
		}
		fmt.Fprintf(&sb, "    %s\n}\n", strings.Join(terms, " && "))
		return sb.String()
	}
	sb.WriteString("    match self {\n")
	for _, v := range decl.Variants {
		n := len(v.Payload)
		if n == 0 {
			fmt.Fprintf(&sb, "        %s.%s -> other is %s.%s\n", name, v.Name, name, v.Name)
			continue
		}
		var mine, theirs, terms []string
		for i, p := range v.Payload {
			mine = append(mine, fmt.Sprintf("a%d", i))
			theirs = append(theirs, fmt.Sprintf("b%d", i))
			// A Copy payload binds by value and needs `&`; a non-Copy payload
			// already binds as a borrow, which `.equals` wants.
			arg := fmt.Sprintf("b%d", i)
			if p.Copy {
				arg = "&" + arg
			}
			terms = append(terms, fmt.Sprintf("a%d.equals(%s)", i, arg))
		}
		fmt.Fprintf(&sb, "        %s.%s(%s) -> other is %s.%s(%s) && %s\n", name, v.Name, strings.Join(mine, ", "), name, v.Name, strings.Join(theirs, ", "), strings.Join(terms, " && "))
	}
	sb.WriteString("    }\n}\n")
	return sb.String()
}

// hashText mixes the parts with FNV-1a; an ADT starts from its tag.
func hashText(decl *Declaration) string {
	var sb strings.Builder
	name := decl.Name
	fmt.Fprintf(&sb, "%spure fn %s.hash(self) -> u64 {\n", protoVis(decl), genericBinder(decl, "Hash"))
	mix := func(indent, expr string) {
		fmt.Fprintf(&sb, "%sh = (h ^ %s).wrappingMul(1099511628211)\n", indent, expr)
	}
	if len(decl.Variants) == 0 {
		sb.WriteString("    let mut h: u64 = 14695981039346656037\n")
		for _, f := range decl.Fields {
			mix("    ", fmt.Sprintf("self.%s.hash()", f.Name))
		}
		sb.WriteString("    h\n}\n")
		return sb.String()
	}
	sb.WriteString("    match self {\n")
	for tag, v := range decl.Variants {
		n := len(v.Payload)
		if n == 0 {
			fmt.Fprintf(&sb, "        %s.%s -> %d\n", name, v.Name, tag+1)
			continue
		}
		var binds []string
		for i := range n {
			binds = append(binds, fmt.Sprintf("p%d", i))
		}
		fmt.Fprintf(&sb, "        %s.%s(%s) -> {\n            let mut h: u64 = %d\n", name, v.Name, strings.Join(binds, ", "), tag+1)
		for i := range n {
			mix("            ", fmt.Sprintf("p%d.hash()", i))
		}
		sb.WriteString("            h\n        }\n")
	}
	sb.WriteString("    }\n}\n")
	return sb.String()
}
