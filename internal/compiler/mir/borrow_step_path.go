package mir

import (
	"slices"

	"kigumi/internal/compiler/sem"
)

// stepPath is a list of steps into a value: a field index (>= 0), a variant payload (payloadStep), or an array
// element (elemAt for a constant index, anyElem otherwise).
type stepPath []int

const (
	anyElem  = -1 << 30
	elemBase = anyElem + 1
	// maxPathLen caps a callee summary's paths; a cut path stands for everything below it.
	maxPathLen = 8
)

// payloadStep keeps payload i apart from field i in a path; payloads of different variants share a step.
func payloadStep(i int) int { return -1 - i }

func elemAt(i int) int { return elemBase + i }

func elemIndex(s int) (int, bool) {
	if s >= elemBase && s < elemBase+1<<29 {
		return s - elemBase, true
	}
	return 0, false
}

func (p stepPath) with(steps ...int) stepPath {
	out := append(slices.Clone(p), steps...)
	if len(out) > maxPathLen {
		out = out[:maxPathLen]
	}
	return out
}

func (p stepPath) commonPrefix(q stepPath) stepPath {
	n := 0
	for n < len(p) && n < len(q) && p[n] == q[n] {
		n++
	}
	return p[:n:n]
}

// fieldPathsOverlap reports whether two field chains of one owner can name overlapping storage:
// one is a prefix of the other, a cut chain standing for its whole last field.
func fieldPathsOverlap(a, b stepPath) bool {
	n := min(len(a), len(b))
	return slices.EqualFunc(a[:n], b[:n], stepsMeet)
}

// stepsMeet: anyElem meets every element step, as an unknown index may be any constant one.
func stepsMeet(a, b int) bool {
	_, ea := elemIndex(a)
	_, eb := elemIndex(b)
	return a == b || a == anyElem && (eb || b == anyElem) || b == anyElem && ea
}

// under puts the step of read in in front of p; an index (unless def finds a constant one) or unbox cuts the path.
func (p stepPath) under(in *Inst, exact bool, def func(LocalID) *Inst) (stepPath, bool) {
	if in.Op == OpIndex && def == nil || in.Op == OpUnbox {
		return nil, false
	}
	step, ok := pathStep(in, def)
	if !ok {
		return nil, false
	}
	return append(stepPath{step}, p...), exact
}

// An unbox has no step; def finds an index operand's definition.
func pathStep(in *Inst, def func(LocalID) *Inst) (int, bool) {
	switch in.Op {
	case OpField, OpFieldMove:
		return in.Index, true
	case OpPayload:
		return payloadStep(in.Index), true
	case OpIndex:
		if def == nil {
			return anyElem, true
		}
		if d := def(in.Args[1]); d != nil && d.Op == OpConst && d.Lit.Kind == sem.LitInt && d.Lit.Int != nil && d.Lit.Int.IsInt64() && d.Lit.Int.Sign() >= 0 && d.Lit.Int.Int64() < 1<<29 {
			return elemAt(int(d.Lit.Int.Int64())), true
		}
		return anyElem, true
	}
	return 0, false
}
