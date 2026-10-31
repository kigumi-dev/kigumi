package printer

import "kigumi/internal/compiler/syntax"

func (p *printer) unsafeGuarantee(id syntax.NodeID) {
	p.write("@unsafeGuarantee(")
	for i, rule := range p.t.List(id).Enumerate() {
		if i > 0 {
			p.write(", ")
		}
		p.ugRule(rule.AsUGRule())
	}
	p.write(")")
}

func (p *printer) ugRule(r syntax.UGRuleView) {
	p.write(r.Property().Text())
	reqs := r.Requirements()
	if !reqs.Valid() {
		return
	}
	p.write(" when ")
	for i, req := range reqs.Enumerate() {
		if i > 0 {
			p.write(" && ")
		}
		p.ugRequirement(req.AsUGRequirement())
	}
}

func (p *printer) ugRequirement(r syntax.UGRequirementView) {
	p.write(r.Param().Text() + ": ")
	for i, prop := range r.Properties().Enumerate() {
		if i > 0 {
			p.write(" + ")
		}
		p.write(prop.Tok().Text())
	}
}
