package sem

import (
	"strings"

	"kigumi/internal/syntax"
)

// ugProp is one Copy/Send/Sync bit of an @unsafeGuarantee rule or requirement.
type ugProp uint8

const (
	ugCopyProp ugProp = 1 << iota
	ugSendProp
	ugSyncProp
)

func ugPropOf(name string) (ugProp, bool) {
	switch name {
	case "Copy":
		return ugCopyProp, true
	case "Send":
		return ugSendProp, true
	case "Sync":
		return ugSyncProp, true
	}
	return 0, false
}

func ugIndex(p ugProp) int {
	switch p {
	case ugCopyProp:
		return 0
	case ugSendProp:
		return 1
	default:
		return 2
	}
}

// ugCond is one `Param: Props` requirement of a rule's `when` clause.
type ugCond struct {
	Param EntityID
	Need  ugProp
}

// ugRule is a guaranteed property, unconditional when Conds is nil.
type ugRule struct {
	Conds []ugCond
}

// UnsafeGuarantee is a type's resolved `@unsafeGuarantee` contract, indexed
// by ugIndex; a nil entry means that property keeps the conservative default.
type UnsafeGuarantee struct {
	Rules [3]*ugRule
}

// extractGuarantee pulls the single @unsafeGuarantee node out of a prefix attrs
// list (0 if absent), erroring on a second one.
func (r *Result) extractGuarantee(f FileID, list syntax.NodeID) syntax.NodeID {
	if list == 0 {
		return 0
	}
	t := r.tree(f)
	var found syntax.NodeID
	for _, a := range t.Children(list) {
		if t.Kind(a) != syntax.UnsafeGuarantee {
			continue
		}
		if found != 0 {
			r.errAt(f, a, cUnsafeGuaranteeDuplicateDirective)
			continue
		}
		found = a
	}
	return found
}

// rejectGuarantee reports a stray @unsafeGuarantee found on a declaration
// that is not a resource/opaque type.
func (r *Result) rejectGuarantee(f FileID, list syntax.NodeID) {
	if ug := r.extractGuarantee(f, list); ug != 0 {
		r.errAt(f, ug, cUnsafeGuaranteeWrongTarget)
	}
}

// declareGuarantee resolves id's @unsafeGuarantee (if any) into info.Guarantee.
func (r *Result) declareGuarantee(id EntityID, node syntax.NodeID) {
	if node == 0 {
		return
	}
	e := &r.Entities[id]
	info := r.typeDecl(id)
	if info.Form != FormResource && info.Form != FormOpaque {
		r.errAt(e.File, node, cUnsafeGuaranteeWrongTarget)
		return
	}
	f := e.File
	t := r.tree(f)
	g := &UnsafeGuarantee{}
	trusted := false
	for _, ruleNode := range t.Children(node) {
		n := t.Node(ruleNode)
		propName := t.TokText(n.Tok)
		prop, ok := ugPropOf(propName)
		if !ok {
			r.errAt(f, ruleNode, cUnsafeGuaranteeUnknownProperty, propName)
			continue
		}
		if prop == ugCopyProp && info.Form == FormResource {
			r.errAt(f, ruleNode, cUnsafeGuaranteeCopyResource)
			continue
		}
		idx := ugIndex(prop)
		if g.Rules[idx] != nil {
			r.errAt(f, ruleNode, cUnsafeGuaranteeDuplicateProperty, propName)
			continue
		}
		g.Rules[idx] = &ugRule{Conds: r.ugConds(f, info, e.Name, syntax.NodeID(n.Lhs))}
		trusted = true
	}
	info.Guarantee = g
	if trusted && !hasSafetyDoc(t, e.Node) {
		r.errAt(f, node, cUnsafeGuaranteeNeedsSafetyDoc)
	}
}

func (r *Result) ugConds(f FileID, info *TypeDeclInfo, typeName string, reqs syntax.NodeID) []ugCond {
	if reqs == 0 {
		return nil
	}
	t := r.tree(f)
	var conds []ugCond
	for _, reqNode := range t.Children(reqs) {
		n := t.Node(reqNode)
		paramName := t.TokText(n.Tok)
		pid := EntityID(0)
		found := false
		for _, p := range info.Params {
			if r.Entities[p].Name == paramName {
				pid, found = p, true
				break
			}
		}
		if !found {
			r.errAt(f, reqNode, cUnsafeGuaranteeUnknownParam, paramName, typeName)
			continue
		}
		var need ugProp
		for _, propNode := range t.Children(syntax.NodeID(n.Lhs)) {
			propName := t.TokText(t.Node(propNode).Tok)
			p, ok := ugPropOf(propName)
			if !ok {
				r.errAt(f, propNode, cUnsafeGuaranteeUnknownProperty, propName)
				continue
			}
			need |= p
		}
		conds = append(conds, ugCond{Param: pid, Need: need})
	}
	return conds
}

// hasSafetyDoc reports whether n's doc comment has a line starting with
// "Safety:", trimmed of leading space.
func hasSafetyDoc(t *syntax.Tree, n syntax.NodeID) bool {
	for _, line := range strings.Split(syntax.DocText(t, n), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Safety:") {
			return true
		}
	}
	return false
}
