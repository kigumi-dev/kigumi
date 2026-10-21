package sem

import (
	"math/big"

	"kigumi/internal/syntax"
	"kigumi/internal/token"
)

// declareConsts runs as pass 5.
func (r *Result) declareConsts() {
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Kind == EntConst && e.File != 0 && e.Flags&EfPoison == 0 {
			r.evalConstDecl(EntityID(id))
		}
	}
}

func (r *Result) evalConstDecl(id EntityID) constValue {
	e := &r.Entities[id]
	info := r.constInfo(id)
	switch info.State {
	case constDone, constFailed:
		return info.Value
	case constEvaluating:
		r.errAt(e.File, e.Node, cConstCycle, e.Name)
		info.State = constFailed
		e.Flags |= EfPoison
		return constValue{}
	}
	info.State = constEvaluating
	t := r.tree(e.File)
	s := constDecl(t, e.Node)
	v, ok := r.evalConst(e.File, s.Value)
	info = r.constInfo(id)
	if !ok {
		info.State = constFailed
		r.Entities[id].Flags |= EfPoison
		return constValue{}
	}
	typ := r.untypedOf(v)
	if s.Type != 0 {
		typ = r.resolveType(e.File, r.fileScopes[e.File], s.Type, posConst)
		if !r.constFits(e.File, s.Value, v, typ) {
			info.State = constFailed
			return constValue{}
		}
	} else if r.Entities[id].Vis.Level != VisPrivate {
		r.errAt(e.File, e.Node, cConstPubNeedsType, e.Name)
	}
	r.Entities[id].Type = typ
	info.Value = v
	info.State = constDone
	return v
}

func (r *Result) untypedOf(v constValue) TypeID {
	switch v.Kind {
	case constInt:
		return TyUntypedInt
	case constFloat:
		return TyUntypedFloat
	case constString:
		return TyString
	case constBytes:
		return TyBytes
	case constBool:
		return TyBool
	case constChar:
		return TyChar
	}
	return TyPoison
}

func (r *Result) intFits(v *big.Int, typ TypeID) bool {
	bits := r.Types.Width(typ)
	if bits == 0 {
		return false
	}
	if r.Types.IsSigned(typ) {
		limit := new(big.Int).Lsh(big.NewInt(1), uint(bits-1))
		return v.Cmp(new(big.Int).Neg(limit)) >= 0 && v.Cmp(limit) < 0
	}
	return v.Sign() >= 0 && v.Cmp(new(big.Int).Lsh(big.NewInt(1), uint(bits))) < 0
}

func (r *Result) evalConst(f FileID, n syntax.NodeID) (constValue, bool) {
	t := r.tree(f)
	node := t.View(n)
	fail := func(what string) (constValue, bool) {
		r.errAt(f, n, cConstNotComptime, what)
		return constValue{}, false
	}
	switch node.Kind() {
	case syntax.IntLit:
		v, ok := syntax.ParseInt(node.Tok().Text())
		if !ok {
			return fail("this literal")
		}
		return constValue{Kind: constInt, Int: v}, true
	case syntax.FloatLit:
		v, ok := syntax.ParseFloat(node.Tok().Text())
		if !ok {
			return fail("this literal")
		}
		return constValue{Kind: constFloat, Float: v}, true
	case syntax.BoolLit:
		return constValue{Kind: constBool, Bool: node.Tok().Kind() == token.KwTrue}, true
	case syntax.CharLit:
		c, ok := syntax.DecodeChar(node.Tok().Text())
		if !ok {
			r.errAt(f, n, cCharInvalid)
			return constValue{}, false
		}
		return constValue{Kind: constChar, Char: c}, true
	case syntax.StringLit:
		if node.AsStringLit().Interps().Valid() {
			return fail("string interpolation")
		}
		raw := node.Tok().Text()
		return constValue{Kind: constString, Str: syntax.DecodeString(raw[1 : len(raw)-1])}, true
	case syntax.ByteStringLit:
		return constValue{Kind: constBytes, Str: string(syntax.DecodeByteString(node.Tok().Text()))}, true
	case syntax.Paren:
		return r.evalConst(f, node.AsParen().Inner().ID())
	case syntax.Ident:
		ent := r.resolveValueName(f, r.fileScopes[f], r.identPath(f, n))
		if ent == 0 {
			return constValue{}, false
		}
		r.Files[f].Uses[n] = ent
		if r.Entities[ent].Kind != EntConst {
			return fail("`" + r.Entities[ent].Name + "`")
		}
		v := r.evalConstDecl(ent)
		return v, v.Kind != constNone
	case syntax.MemberExpr:
		return r.evalQualifiedConst(f, n)
	case syntax.Unary:
		v, ok := r.evalConst(f, node.AsUnary().Operand().ID())
		if !ok {
			return v, false
		}
		return r.constUnary(f, n, node.Tok().Kind(), v)
	case syntax.Binary:
		a, ok := r.evalConst(f, node.AsBinary().Left().ID())
		if !ok {
			return a, false
		}
		b, ok := r.evalConst(f, node.AsBinary().Right().ID())
		if !ok {
			return b, false
		}
		return r.constBinary(f, n, node.Tok().Kind(), a, b)
	}
	return fail("this expression")
}

// identPath returns a Path node for a bare Ident so name resolution can
// share one entry point; the node is created on demand.
func (r *Result) identPath(f FileID, n syntax.NodeID) syntax.NodeID {
	t := r.tree(f)
	if p, ok := r.identPaths[t][n]; ok {
		return p
	}
	tok := t.View(n).Tok().Index()
	p := t.AddPath(tok, []uint32{tok})
	if r.identPaths[t] == nil {
		r.identPaths[t] = map[syntax.NodeID]syntax.NodeID{}
	}
	r.identPaths[t][n] = p
	return p
}
