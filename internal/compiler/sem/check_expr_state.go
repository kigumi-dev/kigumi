package sem

import (
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/token"
)

// exprState is what the checker tracks about the expression it is inside.
type exprState struct {
	// comptimeAt is len(lambdas)+1 inside a comptime block, 0 outside one.
	comptimeAt int
	// comptimeCtx is len(ctxStack) where the innermost comptime block opened; frames below it bind runtime providers.
	comptimeCtx int
	// openStmts counts the statements and block tails in progress; a statement finished inside another must leave the outer variables and touched nodes alone.
	openStmts   int
	touchedBase int
	closureBase int
	// negated holds while a condition is read under an odd number of `!`, where an `is` pattern's bindings are not in scope.
	negated   bool
	loopSpans []token.Span
	// loopValues lists the break values of each bare `for { }`.
	loopValues map[syntax.NodeID][]syntax.NodeID
	// openBorrows are the `&mut` borrows whose referent type is still an open variable.
	openBorrows []scalarBorrow
	// openCaptures wait for their local's type; erased are the closures coerced to a plain fn so far.
	openCaptures []openCapture
	erased       []erasure
	// eraseSites are the values marked lent to a call or bound to a local before they are checked.
	eraseSites map[syntax.NodeID]eraseSite
	// laterArgs are, per lambda argument, the arguments its call evaluates after it but checks before it.
	laterArgs map[syntax.NodeID][]syntax.NodeID
}
