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
	for a := range t.List(attrs).IDs() {
		if t.Kind(a) != syntax.Attr {
			continue
		}
		path := t.View(a).AsAttr().Path()
		if path.Len() != 1 {
			continue
		}
		switch path.Join(".") {
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
			if !cSymbolName(out.LinkName) {
				r.errAt(f, a, cLinkNameBadArg)
				out.LinkName = ""
			}
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
	args := t.View(attr).AsAttr().Args()
	if args.Len() != 1 || args.At(0).Kind() != syntax.StringLit {
		r.errAt(f, attr, code)
		return ""
	}
	v, ok := r.evalConst(f, args.ItemID(0))
	if !ok || v.Kind != constString || v.Str == "" || strings.ContainsRune(v.Str, 0) {
		r.errAt(f, attr, code)
		return ""
	}
	return v.Str
}

func cSymbolName(s string) bool {
	for _, c := range s {
		if !(c == '_' || c == '.' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			return false
		}
	}
	return true
}
