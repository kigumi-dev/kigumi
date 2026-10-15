package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
)

// scalarSig marks the positions a call passes in a native register; a nil entry stays boxed.
type scalarSig struct {
	Params []*scalarInfo
	Ret    *scalarInfo
}

func (s *scalarSig) param(i int) (scalarInfo, bool) {
	if s == nil || i >= len(s.Params) || s.Params[i] == nil {
		return scalarInfo{}, false
	}
	return *s.Params[i], true
}

func (s *scalarSig) ret() (scalarInfo, bool) {
	if s == nil || s.Ret == nil {
		return scalarInfo{}, false
	}
	return *s.Ret, true
}

func (s *scalarSig) empty() bool {
	for _, p := range s.Params {
		if p != nil {
			return false
		}
	}
	return s.Ret == nil
}

// Exports, sections and the entry are entered from C, and an async body from its Future.
func (e *emitter) scalarCarried(f *mir.Func) bool {
	if f.Ent == 0 || f.Async || f == e.p.Entry || !e.r.DirectCarried(f.Ent) || e.r.FnSectionOf(f.Ent) != "" {
		return false
	}
	return e.reachable == nil || e.reachable.Keep(f.Ent)
}

func (e *emitter) scalarSigOf(f *mir.Func) *scalarSig {
	if !e.scalarCarried(f) {
		return nil
	}
	s := &scalarSig{Params: make([]*scalarInfo, len(f.Params))}
	for i, prm := range f.Params {
		l := f.Locals[prm]
		if ty, ok := e.scalarType(l.Type); ok && !l.Cell && e.r.DirectParamSlot(f.Ent, l.Ent) {
			s.Params[i] = &scalarInfo{Ty: ty, Nk: e.numKind(l.Type)}
		}
	}
	if ty, ok := e.scalarType(f.Ret); ok {
		s.Ret = &scalarInfo{Ty: ty, Nk: e.numKind(f.Ret)}
	}
	if s.empty() {
		return nil
	}
	return s
}

// planScalarABI keeps a position native only while the body holds it in a register; dropping one can drop more.
func (e *emitter) planScalarABI() {
	e.ssigs = map[*mir.Func]*scalarSig{}
	var fs []*mir.Func
	for _, f := range e.p.Funcs {
		if s := e.scalarSigOf(f); s != nil {
			e.ssigs[f] = s
			fs = append(fs, f)
		}
	}
	for changed := true; changed; {
		changed = false
		for _, f := range fs {
			changed = e.narrowScalarSig(f) || changed
		}
	}
	for _, f := range fs {
		if e.ssigs[f].empty() {
			delete(e.ssigs, f)
		}
	}
}

func (e *emitter) narrowScalarSig(f *mir.Func) bool {
	s := e.ssigs[f]
	e.direct = e.classifyDirect(f)
	regs := e.classifyScalars(f)
	changed := false
	for i, prm := range f.Params {
		if s.Params[i] != nil && !e.keepsUnboxed(f, prm, regs, s.Ret != nil) {
			s.Params[i], changed = nil, true
		}
	}
	if s.Ret != nil && !e.returnsInRegister(f, regs) {
		s.Ret, changed = nil, true
	}
	return changed
}

func (e *emitter) calleeScalarSig(in mir.Inst) *scalarSig {
	if in.Op != mir.OpCall || in.DPrim != mir.DPNone {
		return nil
	}
	if fn, ok := e.p.ByEnt[in.Ent]; ok {
		return e.ssigs[fn]
	}
	return nil
}

// nativeArg hands a boxed argument over unboxed, releasing the box where the callee or the call site would have.
func (e *emitter) nativeArg(sb *strings.Builder, in mir.Inst, i int, l mir.LocalID, info scalarInfo) string {
	if e.isScalar(l) {
		return e.loadNative(sb, l).render()
	}
	p := e.load(sb, l)
	v := e.unboxScalar(sb, info.Ty, p)
	if !e.p.ParamLent(in.Ent, i) || e.boxTmp[l] {
		e.releaseScalarBox(sb, info.Ty, p)
	}
	return v.render()
}

func (e *emitter) nativeCallInto(sb *strings.Builder, dst mir.LocalID, sym string, args []string, r scalarInfo) {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = call %s %s(%s)\n", t, r.Ty, sym, strings.Join(args, ", "))
	v := Value{r.Ty, t}
	if e.isScalar(dst) {
		e.storeNative(sb, dst, v)
		return
	}
	e.store(sb, dst, e.boxNative(sb, r, v))
}

// callNativeBoxed is callBoxed for a callee with register positions: the adapter unboxes and boxes for it.
func (e *emitter) callNativeBoxed(sb *strings.Builder, f *mir.Func, s *scalarSig, vals []string) string {
	args := make([]string, len(vals))
	for i, v := range vals {
		info, ok := s.param(i)
		if !ok {
			args[i] = "ptr " + v
			continue
		}
		args[i] = e.unboxScalar(sb, info.Ty, v).render()
		if !e.p.ParamLent(f.Ent, i) {
			e.releaseScalarBox(sb, info.Ty, v)
		}
	}
	t := e.newTmp()
	r, ok := s.ret()
	if !ok {
		fmt.Fprintf(sb, "  %s = call ptr %s(%s)\n", t, e.sym(f), strings.Join(args, ", "))
		return t
	}
	fmt.Fprintf(sb, "  %s = call %s %s(%s)\n", t, r.Ty, e.sym(f), strings.Join(args, ", "))
	return e.boxNative(sb, r, Value{r.Ty, t})
}
