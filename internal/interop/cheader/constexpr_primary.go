package cheader

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

func (p *cparser) primary() cval {
	switch t := p.next(); {
	case t == "(":
		v := p.binary(0)
		p.expect(")")
		return v
	case t == "@as":
		p.expect("(")
		typ := p.next()
		p.expect(",")
		v := p.binary(0)
		p.expect(")")
		kt, known := mapScalar(typ, p.abi)
		c, fit := scalarVal(kt, v.v)
		if !known || !fit {
			return p.fail()
		}
		return c
	case t == "__helpers.promoteIntLiteral":
		p.expect("(")
		typ := p.next()
		p.expect(",")
		n := p.binary(0)
		p.expect(",")
		base := p.next()
		p.expect(")")
		return p.promote(typ, n.v, base != ".decimal")
	case suffixMacros[t] && p.peek() == "(":
		p.next()
		arg := p.binary(0)
		p.expect(")")
		if !p.ok {
			return arg
		}
		return p.suffixed(t, arg)
	case t != "" && isIdentStart(t[0]):
		if v, found := p.vals[t]; found {
			return v
		}
		return p.fail()
	case t != "" && t[0] >= '0' && t[0] <= '9':
		v, good := new(big.Int).SetString(t, 0)
		if !good || strings.ContainsAny(t, ".") {
			return p.fail()
		}
		return p.promote("c_int", v, isRadixLiteral(t))
	}
	return p.fail()
}

func isIdentStart(c byte) bool { return c == '_' || (c|0x20 >= 'a' && c|0x20 <= 'z') }

func isRadixLiteral(t string) bool {
	return len(t) > 1 && t[0] == '0' && (strings.ContainsAny(t[1:2], "xXoObB") || !strings.ContainsAny(t, ".eE"))
}

// A hex or octal literal that overflows the signed type takes the unsigned one first, as in C.
func (p *cparser) promote(typ string, v *big.Int, radix bool) cval {
	start, known := mapScalar(typ, p.abi)
	if !known {
		return p.fail()
	}
	order := []string{start, "i64", "u64"}
	if radix && strings.HasPrefix(start, "i") && start != "isize" {
		order = []string{start, "u" + start[1:], "i64", "u64"}
	}
	for _, kt := range order {
		if c, fit := scalarVal(kt, v); fit {
			return c
		}
	}
	return p.fail()
}

func concatStrings(toks []string) (string, bool) {
	var sb strings.Builder
	for i, t := range toks {
		if i%2 == 1 {
			if t != "++" {
				return "", false
			}
			continue
		}
		if len(t) < 2 || t[0] != '"' {
			return "", false
		}
		body, good := kigumiStringBody(t[1 : len(t)-1])
		if !good {
			return "", false
		}
		sb.WriteString(body)
	}
	return `"` + sb.String() + `"`, len(toks)%2 == 1
}

func kigumiStringBody(b string) (string, bool) {
	var sb strings.Builder
	for i := 0; i < len(b); i++ {
		switch c := b[i]; {
		case c == '$':
			sb.WriteString(`\$`)
		case c != '\\':
			sb.WriteByte(c)
		case i+1 >= len(b):
			return "", false
		case b[i+1] == 'x':
			if i+3 >= len(b) {
				return "", false
			}
			n, err := strconv.ParseUint(b[i+2:i+4], 16, 8)
			if err != nil || n >= 0x80 {
				return "", false
			}
			if n == 0 {
				sb.WriteString(`\0`)
			} else {
				fmt.Fprintf(&sb, `\u{%x}`, n)
			}
			i += 3
		case strings.IndexByte(`ntr0\\"'$`, b[i+1]) >= 0:
			sb.WriteString(b[i : i+2])
			i++
		default:
			return "", false
		}
	}
	return sb.String(), true
}

// Integer expressions follow C's integer promotions; strings may be joined by `++`.
func evalConst(target string, abi ABI) (typ, literal string, ok bool) {
	typ, literal, _, ok = evalConstEnv(target, abi, nil)
	return typ, literal, ok
}

func evalConstEnv(target string, abi ABI, vals map[string]cval) (typ, literal string, v cval, ok bool) {
	toks, good := tokenizeConst(target)
	if !good || len(toks) == 0 {
		return "", "", cval{}, false
	}
	if toks[0][0] == '"' {
		s, joined := concatStrings(toks)
		return "String", s, cval{}, joined
	}
	if ft, fl, fok := floatConst(toks, abi); fok {
		return ft, fl, cval{}, true
	}
	p := &cparser{toks: toks, abi: abi, ok: true, vals: vals}
	v = p.binary(0)
	if !p.ok || p.pos != len(toks) {
		return "", "", cval{}, false
	}
	return v.typeName(), spelling(toks, v), v, true
}

// A plain literal keeps its own digits (hex stays hex); computed values print in decimal.
func spelling(toks []string, v cval) string {
	neg := toks[0] == "-"
	if neg {
		toks = toks[1:]
	}
	if (len(toks) == 6 || len(toks) == 8) && (toks[0] == "@as" || toks[0] == "__helpers.promoteIntLiteral") && toks[1] == "(" && toks[3] == "," {
		lit := toks[4]
		if _, single := new(big.Int).SetString(lit, 0); single && (toks[5] == ")" || toks[5] == ",") && (!neg || v.signed) {
			if neg {
				return "-" + lit
			}
			return lit
		}
	}
	return v.v.String()
}

func floatConst(toks []string, abi ABI) (typ, lit string, ok bool) {
	neg := ""
	if len(toks) > 0 && toks[0] == "-" {
		neg, toks = "-", toks[1:]
	}
	if len(toks) != 6 || toks[0] != "@as" || toks[1] != "(" || toks[3] != "," || toks[5] != ")" {
		return "", "", false
	}
	kt, known := mapScalar(toks[2], abi)
	if !known || (kt != "f32" && kt != "f64") || toks[4] == "" || toks[4][0] < '0' || toks[4][0] > '9' {
		return "", "", false
	}
	return kt, neg + toks[4], true
}
