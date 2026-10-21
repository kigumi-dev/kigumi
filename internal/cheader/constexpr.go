package cheader

import (
	"math/big"
	"regexp"
	"strings"
)

var reCTok = regexp.MustCompile(`^(?:"(?:\\.|[^"\\])*"|@as|__helpers\.promoteIntLiteral|0[xXoObB][0-9a-fA-F_]+|[0-9][0-9_]*(?:\.[0-9_]*)?(?:[eE][+-]?[0-9]+)?|\.[a-z]+|[A-Za-z_][A-Za-z_0-9]*|<<|>>|\+\+|[-+*/%&|^~(),])`)

type cparser struct {
	toks []string
	pos  int
	abi  ABI
	ok   bool
	vals map[string]cval
}

func tokenizeConst(s string) ([]string, bool) {
	var toks []string
	for s = strings.TrimSpace(s); s != ""; s = strings.TrimSpace(s) {
		t := reCTok.FindString(s)
		if t == "" {
			return nil, false
		}
		toks = append(toks, t)
		s = s[len(t):]
	}
	return toks, true
}

func (p *cparser) peek() string {
	if p.pos < len(p.toks) {
		return p.toks[p.pos]
	}
	return ""
}

func (p *cparser) next() string {
	t := p.peek()
	p.pos++
	return t
}

func (p *cparser) fail() cval {
	p.ok = false
	return cval{v: new(big.Int)}
}

func (p *cparser) expect(t string) {
	if p.next() != t {
		p.ok = false
	}
}

var binLevels = [][]string{{"|"}, {"^"}, {"&"}, {"<<", ">>"}, {"+", "-"}, {"*", "/", "%"}}

func (p *cparser) binary(level int) cval {
	if level == len(binLevels) {
		return p.unary()
	}
	l := p.binary(level + 1)
	for p.ok && containsStr(binLevels[level], p.peek()) {
		op := p.next()
		r := p.binary(level + 1)
		l = p.apply(op, l, r)
	}
	return l
}

func containsStr(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}
