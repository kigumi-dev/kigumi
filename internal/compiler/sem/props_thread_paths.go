package sem

import "kigumi/internal/compiler/syntax"

// pathCount holds, per way a node ends, the most calls through a parameter on a path ending that way: completing
// (fall), leaving the innermost loop by `break` or `continue`, or leaving the body (exit); noPath marks no such path.
type pathCount struct {
	fall, brk, cont, exit int
}

const noPath = -1

var zeroPath = pathCount{fall: 0, brk: noPath, cont: noPath, exit: noPath}

func fallPath(n int) pathCount {
	return pathCount{fall: min(2, n), brk: noPath, cont: noPath, exit: noPath}
}

func (a pathCount) max() int { return max(a.fall, a.brk, a.cont, a.exit) }

// seqCount runs a then b, which runs only on a's completing path.
func seqCount(a, b pathCount) pathCount {
	after := func(n int) int {
		if a.fall == noPath || n == noPath {
			return noPath
		}
		return min(2, a.fall+n)
	}
	return pathCount{fall: after(b.fall), brk: max(a.brk, after(b.brk)), cont: max(a.cont, after(b.cont)), exit: max(a.exit, after(b.exit))}
}

// branchCount joins mutually exclusive arms: each way out is the busiest arm ending that way.
func branchCount(arms ...pathCount) pathCount {
	out := pathCount{fall: noPath, brk: noPath, cont: noPath, exit: noPath}
	for _, a := range arms {
		out = pathCount{fall: max(out.fall, a.fall), brk: max(out.brk, a.brk), cont: max(out.cont, a.cont), exit: max(out.exit, a.exit)}
	}
	return out
}

// jump ends a's completing path the way a node of kind k that never completes leaves.
func (a pathCount) jump(k syntax.NodeKind) pathCount {
	switch k {
	case syntax.BreakExpr:
		a.brk = max(a.brk, a.fall)
	case syntax.ContinueExpr:
		a.cont = max(a.cont, a.fall)
	default:
		a.exit = max(a.exit, a.fall)
	}
	a.fall = noPath
	return a
}

type pathFn func(v syntax.View, nested bool) pathCount

// loopPathCount: an iteration reaches the next by completing or by `continue`; when no such path calls the parameter,
// every iteration but the last calls nothing, so the loop calls it as often as one iteration leaving it does.
func (r *Result) loopPathCount(v syntax.View, nested bool, path pathFn) pathCount {
	f := v.AsFor()
	head, cond, done := zeroPath, func(bool) pathCount { return zeroPath }, noPath
	switch {
	case f.Pattern().Valid():
		head, done = path(f.Head().View, nested), 0
	case f.Head().Valid():
		cond = func(nested bool) pathCount { return path(f.Head().View, nested) }
		done = cond(nested).fall
	}
	iter := seqCount(cond(nested), path(f.Body().View, nested))
	if max(iter.fall, iter.cont) > 0 {
		again := seqCount(cond(true), path(f.Body().View, true))
		return seqCount(head, fallPath(max(0, again.max())))
	}
	return seqCount(head, pathCount{fall: max(done, iter.brk), brk: noPath, cont: noPath, exit: iter.exit})
}

// matchPathCount: an arm runs after the guards of the arms before it, each of which may have run and failed.
func (r *Result) matchPathCount(m syntax.MatchView, nested bool, path pathFn) pathCount {
	arms := make([]pathCount, 0, m.Arms().Len())
	tried := zeroPath
	for v := range m.Arms().Views() {
		arm := v.AsArm()
		guard := path(arm.Guard().View, nested)
		arms = append(arms, seqCount(tried, seqCount(guard, path(arm.Body().View, nested))))
		tried = seqCount(tried, branchCount(guard, zeroPath))
	}
	return seqCount(path(m.Scrutinee().View, nested), branchCount(arms...))
}
