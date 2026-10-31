package cheader

import "math/big"

func (p *cparser) apply(op string, a, b cval) cval {
	if !p.ok {
		return a
	}
	res := arith(a, b)
	if op == "<<" || op == ">>" {
		res = a.promoted()
		if b.v.Sign() < 0 || b.v.Cmp(big.NewInt(int64(res.bits))) >= 0 {
			return p.fail()
		}
	}
	x, y := new(big.Int).Set(a.v), new(big.Int).Set(b.v)
	z := new(big.Int)
	switch op {
	case "|":
		z.Or(x, y)
	case "^":
		z.Xor(x, y)
	case "&":
		z.And(x, y)
	case "<<":
		z.Lsh(x, uint(y.Int64()))
	case ">>":
		z.Rsh(x, uint(y.Int64()))
	case "+":
		z.Add(x, y)
	case "-":
		z.Sub(x, y)
	case "*":
		z.Mul(x, y)
	case "/", "%":
		if y.Sign() == 0 {
			return p.fail()
		}
		if op == "/" {
			z.Quo(x, y)
		} else {
			z.Rem(x, y)
		}
	}
	out, fit := res.wrap(z)
	if !fit {
		return p.fail()
	}
	return out
}

func (p *cparser) unary() cval {
	switch p.peek() {
	case "-", "~", "+":
		op := p.next()
		a := p.unary().promoted()
		z := new(big.Int).Set(a.v)
		switch op {
		case "-":
			z.Neg(z)
		case "~":
			z.Not(z)
		}
		out, fit := a.wrap(z)
		if !fit {
			return p.fail()
		}
		return out
	}
	return p.primary()
}
