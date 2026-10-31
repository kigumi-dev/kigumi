package sem

import "math/big"

type LitKind uint8

const (
	LitInt LitKind = iota + 1
	LitFloat
	LitString
	LitBool
	LitChar
	LitBytes
	LitRecord
)

type Literal struct {
	Kind   LitKind
	Int    *big.Int
	Float  *big.Float
	Str    string
	Bool   bool
	Char   rune
	Fields []Literal
}

func (v constValue) literal() (Literal, bool) {
	out := Literal{Int: v.Int, Float: v.Float, Str: v.Str, Bool: v.Bool, Char: v.Char}
	switch v.Kind {
	case constInt:
		out.Kind = LitInt
	case constFloat:
		out.Kind = LitFloat
	case constString:
		out.Kind = LitString
	case constBytes:
		out.Kind = LitBytes
	case constBool:
		out.Kind = LitBool
	case constChar:
		out.Kind = LitChar
	case constRecord:
		out.Kind = LitRecord
		for _, f := range v.Fields {
			fl, ok := f.literal()
			if !ok {
				return Literal{}, false
			}
			out.Fields = append(out.Fields, fl)
		}
	default:
		return Literal{}, false
	}
	return out, true
}
