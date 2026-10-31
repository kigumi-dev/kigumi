package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/compiler/mir"
	opdef "kigumi/internal/compiler/mir/ops"
)

var (
	rtDTry    = RuntimeFn{"rt_d_try", []Type{TI32, TI32, TPtr}, TPtr}
	rtDTryOpt = RuntimeFn{"rt_d_try_opt", []Type{TI32, TI32, TPtr, TPtr}, TI32}
)

// tryRefCall runs a try op that hands back its Ok payload bare, or null for AllocError, and builds the Result in place.
func (e *emitter) tryRefCall(sb *strings.Builder, f *mir.Func, in mir.Inst) {
	args, boxed := e.instArgs(sb, in)
	arr := e.argArray(sb, args)
	op, _ := in.StdOp()
	e.std.add(op)
	if in.DPrim == opdef.DPTryOpt {
		out := e.stackSlot(sb, "ptr")
		k := e.irb(sb).Call(rtDTryOpt, vi32(int(op)), vi32(len(args)), vptr(arr), vptr(out)).Text
		e.releaseScalarArgs(sb, in, args)
		e.storeTryOpt(sb, e.direct[in.Dst], aggPtr(in.Dst), k, out)
		e.releaseAggArgs(sb, in, args, boxed)
		return
	}
	h := e.irb(sb).Call(rtDTry, vi32(int(op)), vi32(len(args)), vptr(arr)).Text
	e.releaseScalarArgs(sb, in, args)
	e.storeTryRef(sb, e.direct[in.Dst], aggPtr(in.Dst), h)
	e.releaseAggArgs(sb, in, args, boxed)
}

func (e *emitter) resultTags(a *aggInfo) (ok, err int) {
	if e.r.Entity(a.Variants[0].Ent).Name != "Ok" {
		return 1, 0
	}
	return 0, 1
}

// k is 0 for the refusal, 1 for Ok(None) and 2 for Ok(Some(*out)); a missing reference is zero bits either way.
func (e *emitter) storeTryOpt(sb *strings.Builder, a *aggInfo, base, k, out string) {
	okTag, errTag := e.resultTags(a)
	zeroAgg(sb, a, base)
	refused, tag := e.newTmp(), e.newTmp()
	fmt.Fprintf(sb, "  %s = icmp eq i32 %s, 0\n", refused, k)
	fmt.Fprintf(sb, "  %s = select i1 %s, %s %d, %s %d\n", tag, refused, adtTag, errTag, adtTag, okTag)
	fmt.Fprintf(sb, "  store %s %s, ptr %s\n", adtTag, tag, e.tagPtr(sb, a, base))
	opt := a.Variants[okTag].Payload[0].Agg
	inner := e.slotPtr(sb, a, base, 0)
	some, none := 0, 1
	if len(opt.Variants[0].Payload) == 0 {
		some, none = 1, 0
	}
	has, itag := e.newTmp(), e.newTmp()
	fmt.Fprintf(sb, "  %s = icmp eq i32 %s, 2\n", has, k)
	fmt.Fprintf(sb, "  %s = select i1 %s, %s %d, %s %d\n", itag, has, adtTag, some, adtTag, none)
	fmt.Fprintf(sb, "  store %s %s, ptr %s\n", adtTag, itag, e.tagPtr(sb, opt, inner))
	h := e.newTmp()
	fmt.Fprintf(sb, "  %s = load ptr, ptr %s\n", h, out)
	e.storeRef(sb, e.slotPtr(sb, opt, inner, 0), h)
}

// The Err slot holds AllocError's one payload-free variant, which is all zero bits.
func (e *emitter) storeTryRef(sb *strings.Builder, a *aggInfo, base, h string) {
	okTag, errTag := e.resultTags(a)
	zeroAgg(sb, a, base)
	ok, tag := e.newTmp(), e.newTmp()
	fmt.Fprintf(sb, "  %s = icmp ne ptr %s, null\n", ok, h)
	fmt.Fprintf(sb, "  %s = select i1 %s, %s %d, %s %d\n", tag, ok, adtTag, okTag, adtTag, errTag)
	fmt.Fprintf(sb, "  store %s %s, ptr %s\n", adtTag, tag, e.tagPtr(sb, a, base))
	fld := a.Variants[okTag].Payload[0]
	slot := e.slotPtr(sb, a, base, 0)
	if fld.Ref {
		e.storeRef(sb, slot, h)
		return
	}
	some, done := e.label("tryok"), e.label("trydone")
	fmt.Fprintf(sb, "  br i1 %s, label %%%s, label %%%s\n%s:\n", ok, some, done, some)
	v := e.unboxScalar(sb, fld.Ty, h)
	e.storeLeafScalar(sb, fld, slot, v.Text)
	e.releaseScalarBox(sb, fld.Ty, h)
	fmt.Fprintf(sb, "  br label %%%s\n%s:\n", done, done)
}
