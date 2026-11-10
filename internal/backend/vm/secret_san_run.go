package vm

import (
	"fmt"
	"io"
	"strings"

	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
)

// ShadowValue is the label of the value a SecretSanRun call returned.
type ShadowValue struct {
	// Any is a coarse secret dependence (Bool, aggregate, float, string).
	Any bool
	// Lo and Hi are the bit masks for an integer's secret-dependent bits.
	Lo, Hi uint64
}

// Tainted reports whether any part of the value depends on a secret.
func (v ShadowValue) Tainted() bool { return v.Any || v.Lo != 0 || v.Hi != 0 }

// SecretSanRun runs the lowered function fn on integer arguments, labelling the positions marked secret,
// and returns its result's label; Options.SecretSan is forced on.
func SecretSanRun(prog *mir.Program, opts Options, fn string, vals []int64, secret []bool, out, errOut io.Writer) (ShadowValue, error) {
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = io.Discard
	}
	opts.SecretSan = true
	opts.Trace = false
	m, f, args, err := runFunction(prog, opts, fn, vals, secret, out, errOut)
	if err != nil {
		return ShadowValue{}, err
	}
	var res *obj
	err = m.catch(func() { res = m.call(f, args) })
	if err != nil {
		return ShadowValue{}, err
	}
	s := m.taintOf(res)
	return ShadowValue{Any: s.coarse, Lo: s.lo, Hi: s.hi}, nil
}

// CompareTraces runs fn on a and on b, which differ only in their secret positions, and reports the first trace difference.
func CompareTraces(prog *mir.Program, opts Options, fn string, a, b []int64, out, errOut io.Writer) error {
	if len(a) != len(b) {
		return fmt.Errorf("SecretSan: trace comparison needs equal-length input vectors, got %d and %d", len(a), len(b))
	}
	if out == nil {
		out = io.Discard
	}
	if errOut == nil {
		errOut = io.Discard
	}
	traces := make([]*traceLog, 2)
	for i, vals := range [][]int64{a, b} {
		o := opts
		o.SecretSan = false
		o.Trace = true
		m, f, args, err := runFunction(prog, o, fn, vals, nil, out, errOut)
		if err != nil {
			return err
		}
		if err := m.catch(func() { m.call(f, args) }); err != nil {
			return err
		}
		traces[i] = m.trace
	}
	return traces[0].diff(traces[1])
}

func runFunction(prog *mir.Program, opts Options, fn string, vals []int64, secret []bool, out, errOut io.Writer) (*Machine, *midir.Func, []*obj, error) {
	m := newMachine(prog, opts, out, errOut)
	var f *midir.Func
	for _, cand := range prog.Funcs {
		if cand.Name == fn || strings.HasSuffix(cand.Name, "."+fn) {
			f = cand
			break
		}
	}
	if f == nil {
		return nil, nil, nil, fmt.Errorf("SecretSan: no lowered function named %q", fn)
	}
	if secret != nil && len(secret) != len(vals) {
		return nil, nil, nil, fmt.Errorf("SecretSan: %d secret marks for %d arguments", len(secret), len(vals))
	}
	args, err := m.intArgs(f, vals)
	if err != nil {
		return nil, nil, nil, err
	}
	for i := range args {
		if secret != nil && i < len(secret) && secret[i] {
			nk := m.numKind(f.Locals[f.Params[i]].Type)
			args[i] = m.withTaint(args[i], fullShadow(nk))
		}
	}
	return m, f, args, nil
}

func (m *Machine) intArgs(f *midir.Func, vals []int64) ([]*obj, error) {
	if len(vals) != len(f.Params) {
		return nil, fmt.Errorf("SecretSan: %s takes %d arguments, got %d", f.Name, len(f.Params), len(vals))
	}
	out := make([]*obj, len(vals))
	for i, p := range f.Params {
		t := f.Locals[p].Type
		if !m.r.Types.IsInteger(m.underlyingRepr(t)) {
			return nil, fmt.Errorf("SecretSan: %s argument %d is not an integer", f.Name, i)
		}
		out[i] = mkInt(vals[i], m.numKind(t))
	}
	return out, nil
}
