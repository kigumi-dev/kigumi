package syntax

import "kigumi/internal/token"

// unsafeGuaranteeAttr parses the body of `@unsafeGuarantee(Rule, ...)`; at is the
// already-consumed `@`. Property names and target validity are a sem concern.
func (p *parser) unsafeGuaranteeAttr(at uint32) NodeID {
	p.advance()
	var rules []NodeID
	p.layoutList(token.LParen, token.RParen, func() {
		rules = append(rules, p.ugRule())
	})
	return p.tree.addList(UnsafeGuarantee, at, rules)
}

func (p *parser) ugRule() NodeID {
	prop := p.expect(token.Ident)
	var reqs NodeID
	if p.eatIdent("when") {
		var items []NodeID
		items = append(items, p.ugRequirement())
		for p.at(token.AndAnd) {
			p.advance()
			items = append(items, p.ugRequirement())
		}
		reqs = p.tree.addList(List, prop, items)
	}
	return p.node1(UGRule, prop, reqs)
}

func (p *parser) ugRequirement() NodeID {
	param := p.expect(token.Ident)
	p.expect(token.Colon)
	var props []NodeID
	props = append(props, p.leaf(Ident, p.expect(token.Ident)))
	for p.at(token.Plus) {
		p.advance()
		props = append(props, p.leaf(Ident, p.expect(token.Ident)))
	}
	return p.node1(UGRequirement, param, p.tree.addList(List, param, props))
}
