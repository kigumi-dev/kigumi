package sem

import "kigumi/internal/compiler/syntax"

// CtxBind resolves one required key: a bound provider's field (Local/Field)
// or the current function's own hidden parameter for it (Param).
type CtxBind struct {
	Key   EntityID
	Local EntityID
	Field EntityID
	Param EntityID
}

// ctxBindSource is one key's binding inside a single `context (...)` frame.
type ctxBindSource struct {
	Local EntityID
	Field EntityID
}

// ctxFrame is one `context (...) { }` block's bindings; depth is
// len(checker.lambdas) at push, for capturing into closures opened since.
type ctxFrame struct {
	bind  map[EntityID]ctxBindSource
	depth int
}

// ctxNodeRef names a syntax node a direct context-key use or a call sits at.
type ctxNodeRef struct {
	File FileID
	Node syntax.NodeID
}

// ctxCallEdge is a deferred call-site requirement; lambdas are the closures
// (outermost first) open at the call site, empty outside any closure.
type ctxCallEdge struct {
	ref     ctxNodeRef
	callee  EntityID
	stack   []ctxFrame
	lambdas []EntityID
	// comptime calls see only the frames opened inside their comptime block.
	comptime bool
}

// ctxValueRef is a bare function name used as a value (not called directly),
// checked once the target's final CtxReal set is known.
type ctxValueRef struct {
	ref    ctxNodeRef
	target EntityID
}
