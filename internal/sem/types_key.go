package sem

import (
	"encoding/hex"
	"slices"
	"strings"
)

// TypeKey spells a type with the DeclID of every named type in it.
type TypeKey string

func (r *Result) TypeKey(t TypeID) TypeKey {
	var sb strings.Builder
	r.writeKey(&sb, t, nil)
	return TypeKey(sb.String())
}

// paramNumbering spells a type parameter by its position; nil spells it by name.
type paramNumbering func(EntityID) string

// numberingOf counts a method's receiver binders before its own parameters, which restart their Index at 0.
func (r *Result) numberingOf(fn EntityID) paramNumbering {
	params := r.Fn(fn).TypeParams
	return func(p EntityID) string {
		if i := slices.Index(params, p); i >= 0 {
			return "$" + itoa(i)
		}
		return "$" + itoa(r.typeParam(p).Index)
	}
}

// signatureKey numbers type parameters by position so a signature never names its own declaration.
func (r *Result) signatureKey(fn EntityID, t TypeID) string {
	var sb strings.Builder
	r.writeKey(&sb, t, r.numberingOf(fn))
	return sb.String()
}

func (r *Result) writeKey(sb *strings.Builder, t TypeID, num paramNumbering) {
	n := r.Types.nodes[t]
	switch n.Kind {
	case KNamed, KIface:
		sb.WriteString(r.entityKey(n.Ent))
		if len(n.Args) > 0 {
			sb.WriteString("[")
			r.writeKeys(sb, n.Args, num)
			sb.WriteString("]")
		}
	case KClosure:
		sb.WriteString(r.entityKey(n.Ent))
	case KParam:
		if num != nil {
			sb.WriteString(num(n.Ent))
		} else {
			sb.WriteString(r.entityName(n.Ent))
		}
	case KRef:
		if n.Lifetime != 0 {
			sb.WriteString("&")
			r.writeKey(sb, n.Lifetime, num)
			sb.WriteByte(' ')
			if n.Flags&flagMut != 0 {
				sb.WriteString("mut ")
			}
			r.writeKey(sb, n.Elem, num)
			return
		}
		if n.Flags&flagMut != 0 {
			sb.WriteString("&mut(")
			r.writeKey(sb, n.Elem, num)
			sb.WriteString(")")
			return
		}
		sb.WriteString("&")
		r.writeKey(sb, n.Elem, num)
	case KPtr:
		if n.Flags&flagMut != 0 {
			sb.WriteString("*mut ")
		} else {
			sb.WriteString("*const ")
		}
		r.writeKey(sb, n.Elem, num)
	case KFn:
		sb.WriteString(fnKeyPrefix(n.Flags))
		if n.Var != 0 {
			sb.WriteString("escaping#" + hex.EncodeToString([]byte(r.Types.escSets[n.Var-1])) + " ")
		}
		sb.WriteString("fn(")
		r.writeKeys(sb, n.Args, num)
		if n.Flags&fnCVariadic != 0 {
			sb.WriteString(",...")
		}
		if n.Flags&fnVariadic != 0 {
			sb.WriteString(";variadic")
		}
		sb.WriteString(")->")
		r.writeKey(sb, n.Elem, num)
	default:
		sb.WriteString(strings.ReplaceAll(r.TypeString(t), " ", ""))
	}
}

func (r *Result) writeKeys(sb *strings.Builder, ts []TypeID, num paramNumbering) {
	first := true
	for _, a := range ts {
		if a == 0 {
			continue
		}
		if !first {
			sb.WriteString(",")
		}
		first = false
		r.writeKey(sb, a, num)
	}
}

func (r *Result) entityKey(ent EntityID) string {
	if id := r.DeclID(ent); !id.IsZero() {
		return id.String()
	}
	return r.entityName(ent)
}

func fnKeyPrefix(flags uint16) string {
	var sb strings.Builder
	if flags&fnEscaping != 0 {
		sb.WriteString("escaping ")
	}
	if flags&fnCAbi != 0 {
		sb.WriteString("extern(C) ")
	}
	eff := Effects(flags)
	if flags&fnEffectPoly != 0 {
		sb.WriteString("pure? ")
		eff &^= EffPure
	}
	for _, e := range []struct {
		bit  Effects
		name string
	}{{EffPure, "pure "}, {EffNoalloc, "noalloc "}, {EffAsync, "async "}, {EffUnsafe, "unsafe "}} {
		if eff&e.bit != 0 {
			sb.WriteString(e.name)
		}
	}
	return sb.String()
}
