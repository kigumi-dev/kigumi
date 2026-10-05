package sem

import "kigumi/internal/syntax"

// ffiFreeLayoutFns move a T through raw C memory themselves, like CValue,
// instead of generated per-instantiation code, so T needs the same gate.
var ffiFreeLayoutFns = map[string]bool{"sizeOf": true, "alignOf": true, "readRecord": true, "writeRecord": true}

// llgen emits one shared body per generic function (not per instantiation),
// so a type parameter here is rejected outright rather than deferred.
func (c *checker) ffiCValueCheck(n syntax.NodeID, fn EntityID, inst []TypeID, recv TypeID) {
	e := &c.r.Entities[fn]
	info := c.r.Fn(fn)
	if c.r.Packages[e.Pkg].Path != "std/ffi" {
		return
	}
	isCValue := info.Owner != 0 && c.r.Entities[info.Owner].Name == "CValue"
	var t TypeID
	switch {
	case isCValue && e.Name == "new":
		if len(inst) == 0 {
			return
		}
		t = c.vars.resolve(inst[0])
	case isCValue && (e.Name == "get" || e.Name == "set"):
		rt := c.vars.resolve(recv)
		if c.r.Types.Kind(rt) == KRef {
			rt = c.vars.resolve(c.r.Types.Node(rt).Elem)
		}
		rn := c.r.Types.Node(rt)
		if rn.Kind != KNamed || len(rn.Args) == 0 {
			return
		}
		t = c.vars.resolve(rn.Args[0])
	case info.Owner == 0 && ffiFreeLayoutFns[e.Name]:
		if len(inst) == 0 {
			return
		}
		t = c.vars.resolve(inst[0])
	default:
		return
	}
	tt := c.r.Types
	if t == TyPoison || tt.Kind(t) == KVar || c.cValueScalar(t) || c.r.cRecord(t) {
		return
	}
	if isCValue {
		c.errAt(n, cCValueType, c.r.TypeString(t))
		return
	}
	c.errAt(n, cFFILayoutType, c.r.TypeString(t), e.Name)
}

// ffiAddressLeaseCheck rejects a withAddress(Const) call whose callback
// could hand its own lease pointer back as R, letting it outlive the call.
func (c *checker) ffiAddressLeaseCheck(n syntax.NodeID, fn EntityID, inst []TypeID) {
	e := &c.r.Entities[fn]
	if c.r.Packages[e.Pkg].Path != "std/ffi" || (e.Name != "withAddress" && e.Name != "withAddressConst") {
		return
	}
	if len(inst) < 2 {
		return
	}
	t := c.vars.resolve(inst[0])
	if t != TyPoison && t != 0 && !c.cValueScalar(t) {
		c.errAt(n, cAddressLeaseType, c.r.TypeString(t), e.Name)
		return
	}
	ret := c.vars.resolve(inst[1])
	if !c.r.mayContainPtr(ret, map[TypeID]bool{}) {
		return
	}
	c.errAt(n, cCarrierLeaseEscape, e.Name, c.r.TypeString(ret))
}

// Int and Float are aliases already covered by IsNumeric; no separate check needed.
func (c *checker) cValueScalar(t TypeID) bool {
	tt := c.r.Types
	if tt.IsNumeric(t) || t == TyBool || t == TyChar || tt.Kind(t) == KPtr || tt.IsCFn(t) {
		return true
	}
	if n := tt.Node(t); n.Kind == KNamed && c.r.typeDecl(n.Ent).Form == FormDefined {
		return c.cValueScalar(c.r.underlyingRepr(t))
	}
	return false
}
