package schema

// Contracts is a function or method's effect/operation summary, read from
// sem rather than re-derived from source text.
type Contracts struct {
	DeclaredPure    bool `json:"declaredPure,omitempty"`
	DeclaredNoalloc bool `json:"declaredNoalloc,omitempty"`
	DeclaredAsync   bool `json:"declaredAsync,omitempty"`
	DeclaredUnsafe  bool `json:"declaredUnsafe,omitempty"`
	// InferredPure/InferredNoalloc are the checker's own answer, which may
	// be stricter than Declared for an undeclared body.
	InferredPure    bool `json:"inferredPure,omitempty"`
	InferredNoalloc bool `json:"inferredNoalloc,omitempty"`
	// Operations lists the OpFact names (internal/compiler/sem's closed vocabulary:
	// "allocate", "block", "panic", ...) the body may perform.
	Operations []string `json:"operations,omitempty"`
	// Unresolved marks a body that still depends on a caller's concrete
	// argument to settle its own summary.
	Unresolved bool `json:"unresolved,omitempty"`
}
