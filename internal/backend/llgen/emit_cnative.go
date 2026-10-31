package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
)

// cForeign reports a call to a fixed-arity C function that takes no record by value.
func (e *emitter) cForeign(in mir.Inst) bool {
	if in.Op != mir.OpCall || in.DPrim != mir.DPNone || e.r.Entity(in.Ent).Kind != sem.EntFn {
		return false
	}
	if _, ok := e.p.ByEnt[in.Ent]; ok {
		return false
	}
	info := e.r.Fn(in.Ent)
	return info.Abi != "" && !info.Naked && !e.byValue(info.Sig) && e.r.Types.Node(info.Sig).Flags&sem.FnCVariadic == 0
}

// cNative reports a call whose C scalar arguments and result stay in registers.
func (e *emitter) cNative(in mir.Inst) bool { return e.cForeign(in) || e.ffiRegOp(in) != "" }

func (e *emitter) cArgInReg(in mir.Inst, i int) bool {
	if e.ffiRegOp(in) != "" {
		return true
	}
	args := e.r.Types.Node(e.r.Fn(in.Ent).Sig).Args
	return i < len(args) && e.cRegister(args[i])
}

func (e *emitter) cDstInReg(f *mir.Func, in mir.Inst) bool {
	if e.ffiRegOp(in) != "" {
		_, ok := e.scalarType(f.Locals[in.Dst].Type)
		return ok
	}
	return e.cRegister(e.r.Types.Node(e.r.Fn(in.Ent).Sig).Elem)
}

func (e *emitter) tryCNativeCall(sb *strings.Builder, f *mir.Func, in mir.Inst) bool {
	if key := e.ffiRegOp(in); key != "" {
		e.ffiRegCall(sb, f, in, key)
		return true
	}
	if !e.cForeign(in) {
		return false
	}
	sn := e.r.Types.Node(e.r.Fn(in.Ent).Sig)
	if len(in.Args) != len(sn.Args) {
		panic(fmt.Sprintf("llgen: call to C function %s passes %d arguments for %d parameters", e.cName(in.Ent), len(in.Args), len(sn.Args)))
	}
	ops := newCOperands(in)
	params := make([]string, len(sn.Args))
	vals := make([]string, len(sn.Args))
	for i, pt := range sn.Args {
		params[i] = e.cType(pt)
		vals[i] = params[i] + e.cExt(pt) + " " + ops.c(e, sb, i, pt, params[i])
	}
	ret := e.cType(sn.Elem)
	call := fmt.Sprintf("call%s %s (%s) %s(%s)", e.cExt(sn.Elem), ret, strings.Join(params, ", "), e.declareForeign(in.Ent), strings.Join(vals, ", "))
	if ret == "void" {
		fmt.Fprintf(sb, "  %s\n", call)
		e.storeMaybeScalar(sb, in.Dst, e.irb(sb).Call(rtUnit).Text)
	} else {
		raw := e.newTmp()
		fmt.Fprintf(sb, "  %s = %s\n", raw, call)
		e.storeCResult(sb, in.Dst, raw, sn.Elem, ret)
	}
	ops.release(e, sb)
	return true
}

func (e *emitter) storeCResult(sb *strings.Builder, dst mir.LocalID, raw string, t sem.TypeID, ct string) {
	if e.isScalar(dst) {
		e.storeNative(sb, dst, e.cWiden(sb, raw, t, ct))
		return
	}
	e.store(sb, dst, e.box(sb, raw, t, ct))
}

// cOperands reads a C call's arguments, remembering the boxes it loaded so a crossing's temporary is released as the boxed path would.
type cOperands struct {
	in     mir.Inst
	loaded []string
	boxed  []bool
}

func newCOperands(in mir.Inst) *cOperands {
	return &cOperands{in: in, loaded: make([]string, len(in.Args)), boxed: make([]bool, len(in.Args))}
}

func (o *cOperands) load(e *emitter, sb *strings.Builder, i int) string {
	l := o.in.Args[i]
	o.loaded[i], o.boxed[i] = e.load(sb, l), e.boxTmp[l]
	return o.loaded[i]
}

// c is argument i as the C type ct of parameter type pt.
func (o *cOperands) c(e *emitter, sb *strings.Builder, i int, pt sem.TypeID, ct string) string {
	if l := o.in.Args[i]; e.isScalar(l) {
		return e.cNarrow(sb, e.loadNative(sb, l), pt, ct)
	}
	return e.unbox(sb, o.load(e, sb, i), pt, ct)
}

// reg is argument i in a register of type ty.
func (o *cOperands) reg(e *emitter, sb *strings.Builder, i int, ty Type) Value {
	if l := o.in.Args[i]; e.isScalar(l) {
		return e.loadNative(sb, l)
	}
	return e.unboxScalar(sb, ty, o.load(e, sb, i))
}

func (o *cOperands) release(e *emitter, sb *strings.Builder) {
	e.releaseAggArgs(sb, o.in, o.loaded, o.boxed)
}
