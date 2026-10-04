package sem

import (
	"strings"

	"kigumi/internal/syntax"
)

// linkSectionResult is a declaration's resolved `@linkName`/`@section`
// bare attributes.
type linkSectionResult struct {
	LinkName string
	Section  string
}

// resolveLinkSection validates `@linkName`/`@section` on attrs against the caller's allow flags; metaAttrs skips both names, so this is their only validation.
func (r *Result) resolveLinkSection(f FileID, attrs syntax.NodeID, allowLink, allowSection bool) linkSectionResult {
	var out linkSectionResult
	if attrs == 0 {
		return out
	}
	t := r.tree(f)
	sawLink, sawSection := false, false
	for _, a := range t.Children(attrs) {
		if t.Kind(a) != syntax.Attr {
			continue
		}
		path := syntax.NodeID(t.Nodes[a].Lhs)
		if len(t.PathToks(path)) != 1 {
			continue
		}
		switch pathText(t, path, ".") {
		case "linkName":
			if !allowLink {
				r.errAt(f, a, cLinkNameBadTarget)
				continue
			}
			if sawLink {
				r.errAt(f, a, cAttrDuplicate, "linkName")
				continue
			}
			sawLink = true
			out.LinkName = r.oneStringArg(f, a, cLinkNameBadArg)
		case "section":
			if !allowSection {
				r.errAt(f, a, cSectionBadTarget)
				continue
			}
			if sawSection {
				r.errAt(f, a, cAttrDuplicate, "section")
				continue
			}
			sawSection = true
			out.Section = r.oneStringArg(f, a, cSectionBadArg)
		}
	}
	return out
}

// oneStringArg reads a bare attribute's sole nonempty, NUL-free string
// literal argument, reporting code on anything else.
func (r *Result) oneStringArg(f FileID, attr syntax.NodeID, code Code) string {
	t := r.tree(f)
	args := t.Children(syntax.NodeID(t.Nodes[attr].Rhs))
	if len(args) != 1 || t.Kind(args[0]) != syntax.StringLit {
		r.errAt(f, attr, code)
		return ""
	}
	v, ok := r.evalConst(f, args[0])
	if !ok || v.Kind != constString || v.Str == "" || strings.ContainsRune(v.Str, 0) {
		r.errAt(f, attr, code)
		return ""
	}
	return v.Str
}
