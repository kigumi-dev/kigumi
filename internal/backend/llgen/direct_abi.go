package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/compiler/mir"
)

// aggByValueBytes: up to two machine words travels as a first-class value,
// above it as a pointer to caller storage the callee copies in.
const aggByValueBytes = 16

func byValueAgg(a *aggInfo) bool { return a.Size <= aggByValueBytes && !a.holdsArray() }

// A nil entry means that position stays a val.
type aggSig struct {
	Params []*aggInfo
	Ret    *aggInfo
}

func (s *aggSig) param(i int) *aggInfo {
	if s == nil || i >= len(s.Params) {
		return nil
	}
	return s.Params[i]
}

func (e *emitter) directSig(f *mir.Func) *aggSig {
	if s, ok := e.sigs[f]; ok {
		return s
	}
	s := e.computeSig(f)
	e.sigs[f] = s
	return s
}

// computeSig reads the signature MIR fixed, so a call site and the
// definition always agree.
func (e *emitter) computeSig(f *mir.Func) *aggSig {
	d := f.DSig
	if d == nil {
		return nil
	}
	s := &aggSig{Params: make([]*aggInfo, len(f.Params))}
	for i := range f.Params {
		if t := d.Param(i); t != 0 {
			s.Params[i], _ = e.agg(t)
		}
	}
	if d.Ret != 0 {
		s.Ret, _ = e.agg(d.Ret)
	}
	return s
}

func (e *emitter) sigHeader(f *mir.Func, s *aggSig) string {
	ret, params := "ptr", []string{}
	ss := e.ssigs[f]
	if r, ok := ss.ret(); ok {
		ret = r.Ty.String()
	}
	if s != nil && s.Ret != nil {
		if byValueAgg(s.Ret) {
			ret = s.Ret.Name
		} else {
			ret, params = "void", append(params, "ptr %sret")
		}
	}
	from := 0
	if env, ok := e.envOf(f); ok {
		params, from = append(params, "ptr %env"), len(env.Fields)
	}
	for i := from; i < len(f.Params); i++ {
		if info, ok := ss.param(i); ok {
			params = append(params, fmt.Sprintf("%s %%p%d", info.Ty, i))
			continue
		}
		if a := s.param(i); a != nil && byValueAgg(a) {
			params = append(params, fmt.Sprintf("%s %%p%d", a.Name, i))
			continue
		}
		params = append(params, fmt.Sprintf("ptr %%p%d", i))
	}
	return fmt.Sprintf("define %s %s(%s)", ret, e.sym(f), strings.Join(params, ", "))
}

func (e *emitter) storeParams(sb *strings.Builder, f *mir.Func, s *aggSig) {
	from := 0
	if env, ok := e.envOf(f); ok {
		for i, fld := range env.Fields {
			e.loadLeaf(sb, f, fld, e.fieldPtr(sb, env, "%env", i), f.Params[i])
		}
		from = len(env.Fields)
	}
	for i := from; i < len(f.Params); i++ {
		prm := f.Params[i]
		if info, ok := e.ssigs[f].param(i); ok {
			if !e.isScalar(prm) {
				panic(fmt.Sprintf("llgen: %s: register parameter %%l%d is boxed in the body", f.Name, prm))
			}
			fmt.Fprintf(sb, "  store %s %%p%d, ptr %%l%d\n", info.Ty, i, prm)
			continue
		}
		a := s.param(i)
		if a == nil {
			fmt.Fprintf(sb, "  store ptr %%p%d, ptr %%l%d\n", i, prm)
			continue
		}
		e.requireDirect(prm, "parameter")
		if byValueAgg(a) {
			fmt.Fprintf(sb, "  store %s %%p%d, ptr %s\n", a.Name, i, aggPtr(prm))
			continue
		}
		copyAgg(sb, a, aggPtr(prm), fmt.Sprintf("%%p%d", i))
	}
}
