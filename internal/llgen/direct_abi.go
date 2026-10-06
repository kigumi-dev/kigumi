package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

// aggByValueBytes: up to two machine words travels as a first-class value,
// above it as a pointer to caller storage the callee copies in.
const aggByValueBytes = 16

func byValueAgg(a *aggInfo) bool { return a.Size <= aggByValueBytes }

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

// computeSig stays a function of the declaration alone, so a call site and
// the definition always agree on the signature.
func (e *emitter) computeSig(f *mir.Func) *aggSig {
	if f.Async || f == e.p.Entry || e.r.Entity(f.Ent).Kind != sem.EntFn {
		return nil
	}
	info := e.r.Fn(f.Ent)
	if info.Naked || info.Export != "" || info.Abi != "" {
		return nil
	}
	s := &aggSig{Params: make([]*aggInfo, len(f.Params))}
	any := false
	for i, p := range f.Params {
		l := f.Locals[p]
		if l.Cell || l.Borrowed {
			continue
		}
		if a, ok := e.agg(l.Type); ok {
			s.Params[i], any = a, true
		}
	}
	if a, ok := e.aggReturn(f); ok {
		s.Ret, any = a, true
	}
	if !any {
		return nil
	}
	return s
}

func (e *emitter) aggReturn(f *mir.Func) (*aggInfo, bool) {
	var found *aggInfo
	for _, blk := range f.Blocks {
		if blk.Term.Op != mir.TermReturn {
			continue
		}
		a, ok := e.agg(f.Locals[blk.Term.Args[0]].Type)
		if !ok || (found != nil && found != a) {
			return nil, false
		}
		found = a
	}
	return found, found != nil
}

func (e *emitter) sigHeader(f *mir.Func, s *aggSig) string {
	ret, params := "ptr", []string{}
	if s != nil && s.Ret != nil {
		if byValueAgg(s.Ret) {
			ret = s.Ret.Name
		} else {
			ret, params = "void", append(params, "ptr %sret")
		}
	}
	for i := range f.Params {
		if a := s.param(i); a != nil && byValueAgg(a) {
			params = append(params, fmt.Sprintf("%s %%p%d", a.Name, i))
			continue
		}
		params = append(params, fmt.Sprintf("ptr %%p%d", i))
	}
	return fmt.Sprintf("define %s %s(%s)", ret, e.sym(f), strings.Join(params, ", "))
}

func (e *emitter) storeParams(sb *strings.Builder, f *mir.Func, s *aggSig) {
	for i, prm := range f.Params {
		a := s.param(i)
		if a == nil {
			fmt.Fprintf(sb, "  store ptr %%p%d, ptr %%l%d\n", i, prm)
			continue
		}
		v := fmt.Sprintf("%%p%d", i)
		if !byValueAgg(a) {
			v = fmt.Sprintf("%%pv%d", i)
			fmt.Fprintf(sb, "  %s = load %s, ptr %%p%d\n", v, a.Name, i)
		}
		if e.isDirect(prm) {
			fmt.Fprintf(sb, "  store %s %s, ptr %%l%d\n", a.Name, v, prm)
			continue
		}
		fmt.Fprintf(sb, "  store ptr %s, ptr %%l%d\n", e.boxAgg(sb, a, v), prm)
	}
}
