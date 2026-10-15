package sem

import "kigumi/internal/syntax"

// ffiFreeLayoutFns move a T through raw C memory themselves, like CValue,
// instead of generated per-instantiation code, so T needs the same gate.
var ffiFreeLayoutFns = map[string]bool{"sizeOf": true, "alignOf": true, "readRecord": true, "writeRecord": true}

type ffiValueCheck struct {
	node syntax.NodeID
	fn   EntityID
	inst []TypeID
	recv TypeID
}

// A function value's type arguments are known only once inference settles, so its gates run then.
func (c *checker) deferFFIValueCheck(n syntax.NodeID, fn EntityID, inst []TypeID, recv TypeID) {
	if c.r.Packages[c.r.Entities[fn].Pkg].Path == "std/ffi" {
		c.ffiValues = append(c.ffiValues, ffiValueCheck{n, fn, inst, recv})
	}
}

func (c *checker) runFFIValueChecks() {
	pending := c.ffiValues
	c.ffiValues = nil
	for _, v := range pending {
		c.ffiCValueCheck(v.node, v.fn, v.inst, v.recv)
		c.ffiAddressLeaseCheck(v.node, v.fn, v.inst)
	}
}

// A generic body is checked once for all its type arguments, so a type parameter here is rejected rather than deferred.
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
		c.errAt(n, c.r.abiCode(t, cAddressLeaseType), c.r.TypeString(t), e.Name)
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
	if c.r.cNumeric(t) || t == TyBool || t == TyChar || tt.Kind(t) == KPtr || tt.IsCFn(t) {
		return true
	}
	if n := tt.Node(t); n.Kind == KNamed && c.r.typeDecl(n.Ent).Form == FormDefined {
		return c.cValueScalar(c.r.underlyingRepr(t))
	}
	return false
}
