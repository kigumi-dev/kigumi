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
	case t == "__helpers.cast":
		p.expect("(")
		typ := p.next()
		p.expect(",")
		v := p.binary(0)
		p.expect(")")
		kt, known := mapScalar(typ, p.abi)
		if !known {
			// A pointer target is C's NULL (or another pointer constant);
			// Kigumi has no null pointer constant to spell it with.
			return p.fail()
		}
		c, fit := scalarVal(kt, v.v)
		if !fit {
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
	case t != "" && t[0] == '\'':
		v, ok := charLiteralValue(t)
		if !ok {
			return p.fail()
		}
		return p.promote("c_int", v, false)
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

// charLiteralValue evaluates one C character constant (`'A'`, `'\n'`, `'\x41'`),
// whose type in C is int.
func charLiteralValue(tok string) (*big.Int, bool) {
	if len(tok) < 3 || tok[0] != '\'' || tok[len(tok)-1] != '\'' {
		return nil, false
	}
	body := tok[1 : len(tok)-1]
	if body == "" {
		return nil, false
	}
	var n int64
	if body[0] != '\\' {
		if len(body) != 1 || body[0] >= 0x80 {
			return nil, false
		}
		n = int64(body[0])
	} else {
		esc := body[1:]
		switch {
		case len(esc) == 1:
			switch esc[0] {
			case 'n':
				n = 10
			case 't':
				n = 9
			case 'r':
				n = 13
			case 'a':
				n = 7
			case 'b':
				n = 8
			case 'f':
				n = 12
			case 'v':
				n = 11
			case '\\', '\'', '"', '?':
				n = int64(esc[0])
			case '0':
				n = 0
			default:
				return nil, false
			}
		case len(esc) > 1 && esc[0] == 'x':
			x, err := strconv.ParseInt(esc[1:], 16, 64)
			if err != nil || x < 0 || x > 0xff {
				return nil, false
			}
			n = x
		case len(esc) > 1 && esc[0] >= '0' && esc[0] <= '7':
			x, err := strconv.ParseInt(esc, 8, 64)
			if err != nil || x < 0 || x > 0xff {
				return nil, false
			}
			n = x
		default:
			return nil, false
		}
	}
	return big.NewInt(n), true
}

// C's integer conversion rank picks a literal's type: an unsigned start
// stays unsigned (a `u` suffix's literal climbs u32 -> u64), a hex or octal
// literal may cross into unsigned, and a decimal one never does.
func (p *cparser) promote(typ string, v *big.Int, radix bool) cval {
	start, known := mapScalar(typ, p.abi)
	if !known {
		return p.fail()
	}
	var order []string
	switch {
	case strings.HasPrefix(start, "u"):
		order = []string{start, "u64"}
	case start == "isize":
		order = []string{"isize", "usize"}
	case radix:
		order = []string{start, "u" + start[1:], "i64", "u64"}
	default:
		// The decimal fallback keeps zig's UINT64_MAX spelling
		// (`__UINT64_C(promoteIntLiteral(c_int, 18446744073709551615, ...))`)
		// working; its suffix macro pins the unsigned type.
		order = []string{start, "i64", "u64"}
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
