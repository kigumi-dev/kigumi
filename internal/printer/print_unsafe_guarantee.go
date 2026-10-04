package printer

import "kigumi/internal/syntax"

func (p *printer) unsafeGuarantee(id syntax.NodeID) {
	p.write("@unsafeGuarantee(")
	for i, rule := range p.t.Children(id) {
		if i > 0 {
			p.write(", ")
		}
		p.ugRule(rule)
	}
	p.write(")")
}

func (p *printer) ugRule(id syntax.NodeID) {
	n := p.node(id)
	p.write(p.tok(n.Tok))
	if n.Lhs == 0 {
		return
	}
	p.write(" when ")
	for i, req := range p.t.Children(syntax.NodeID(n.Lhs)) {
		if i > 0 {
			p.write(" && ")
		}
		p.ugRequirement(req)
	}
}

func (p *printer) ugRequirement(id syntax.NodeID) {
	n := p.node(id)
	p.write(p.tok(n.Tok) + ": ")
	for i, prop := range p.t.Children(syntax.NodeID(n.Lhs)) {
		if i > 0 {
			p.write(" + ")
		}
		p.write(p.tok(p.node(prop).Tok))
	}
}
