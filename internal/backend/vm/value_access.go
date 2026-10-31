package vm

import (
	"math"
	"unsafe"

	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

func (v *obj) nkind() int   { return int(v.numk) }
func (v *obj) setNk(nk int) { v.numk = int16(nk) }

func (v *obj) fl() float64     { return math.Float64frombits(uint64(v.i)) }
func (v *obj) setFl(f float64) { v.i = int64(math.Float64bits(f)) }

func (v *obj) ch() rune     { return rune(v.i) }
func (v *obj) setCh(c rune) { v.i = int64(c) }

func (v *obj) hiWord() int64 { return v.w }
func (v *obj) setHi(h int64) { v.w = h }

func (v *obj) charged() int64 {
	switch v.k {
	case kInt, kFloat, kChar:
		return 0
	}
	return v.w
}

func (v *obj) setCharged(n int64) { v.w = n }

func (v *obj) dynType() sem.TypeID {
	switch v.k {
	case kBox:
		return sem.TypeID(v.i)
	case kClosure:
		return v.clo().dyn
	}
	return 0
}

func (v *obj) str() []byte {
	if v.k == kStr || v.k == kBytes {
		return (*strObj)(unsafe.Pointer(v)).s
	}
	return nil
}

func (v *obj) setStr(s []byte) {
	if v.k != kStr && v.k != kBytes {
		panic("vm: setStr on a value that is not a string or bytes")
	}
	(*strObj)(unsafe.Pointer(v)).s = s
}

var (
	noClosure closureObj
	noOpaque  opaqueObj
)

func (v *obj) clo() *closureObj {
	if v.k != kClosure {
		return &noClosure
	}
	return (*closureObj)(unsafe.Pointer(v))
}

func (v *obj) cfn() *midir.Func          { return v.clo().fn }
func (v *obj) cenv() *obj                { return v.clo().env }
func (v *obj) creq() *reqCall            { return v.clo().req }
func (v *obj) cstd() string              { return v.clo().stdKey }
func (v *obj) clent() int                { return v.clo().lent }
func (v *obj) cprov() opdef.ProviderOpId { return v.clo().prov }

func (v *obj) opq() *opaqueObj {
	if v.k != kOpaque {
		return &noOpaque
	}
	return (*opaqueObj)(unsafe.Pointer(v))
}

func (v *obj) opaqueOp() string { return v.opq().op }
func (v *obj) opaqueData() any  { return v.opq().data }

func dropExtension(v *obj) {
	switch v.k {
	case kStr, kBytes:
		v.setStr(nil)
	case kClosure:
		*v.clo() = closureObj{obj: *v}
	case kOpaque:
		*v.opq() = opaqueObj{obj: *v}
	}
}
