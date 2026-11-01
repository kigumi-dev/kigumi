package ownership

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// Path is a list of steps into a value: a field index (>= 0), a variant payload (PayloadStep), or an array
// element (ElemAt for a constant index, AnyElem otherwise).
type Path []int

const (
	AnyElem  = -1 << 30
	elemBase = AnyElem + 1
	// MaxPathLen caps a callee summary's paths; a cut path stands for everything below it.
	MaxPathLen = 8
)

// PayloadStep keeps payload i apart from field i in a path; payloads of different variants share a step.
func PayloadStep(i int) int { return -1 - i }

func ElemAt(i int) int { return elemBase + i }

func ElemIndex(s int) (int, bool) {
	if s >= elemBase && s < elemBase+1<<29 {
		return s - elemBase, true
	}
	return 0, false
}

func (p Path) With(steps ...int) Path {
	out := append(slices.Clone(p), steps...)
	if len(out) > MaxPathLen {
		out = out[:MaxPathLen]
	}
	return out
}

func (p Path) CommonPrefix(q Path) Path {
	n := 0
	for n < len(p) && n < len(q) && p[n] == q[n] {
		n++
	}
	return p[:n:n]
}

// FieldPathsOverlap reports whether two field chains of one owner can name overlapping storage:
// one is a prefix of the other, a cut chain standing for its whole last field.
func FieldPathsOverlap(a, b Path) bool {
	n := min(len(a), len(b))
	return slices.EqualFunc(a[:n], b[:n], StepsMeet)
}

// StepsMeet: AnyElem meets every element step, as an unknown index may be any constant one.
func StepsMeet(a, b int) bool {
	_, ea := ElemIndex(a)
	_, eb := ElemIndex(b)
	return a == b || a == AnyElem && (eb || b == AnyElem) || b == AnyElem && ea
}

// Under puts the step of read in in front of p; an index (unless def finds a constant one) or unbox cuts the path.
func (p Path) Under(in *midir.Inst, exact bool, def func(midir.LocalID) *midir.Inst) (Path, bool) {
	if in.Op == midir.OpIndex && def == nil || in.Op == midir.OpUnbox {
		return nil, false
	}
	step, ok := PathStep(in, def)
	if !ok {
		return nil, false
	}
	return append(Path{step}, p...), exact
}

// An unbox has no step; def finds an index operand's definition.
func PathStep(in *midir.Inst, def func(midir.LocalID) *midir.Inst) (int, bool) {
	switch in.Op {
	case midir.OpField, midir.OpFieldMove:
		return in.Index, true
	case midir.OpPayload:
		return PayloadStep(in.Index), true
	case midir.OpIndex:
		if def == nil {
			return AnyElem, true
		}
		if d := def(in.Args[1]); d != nil && d.Op == midir.OpConst && d.Lit.Kind == sem.LitInt && d.Lit.Int != nil && d.Lit.Int.IsInt64() && d.Lit.Int.Sign() >= 0 && d.Lit.Int.Int64() < 1<<29 {
			return ElemAt(int(d.Lit.Int.Int64())), true
		}
		return AnyElem, true
	}
	return 0, false
}
