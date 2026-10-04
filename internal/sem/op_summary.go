package sem

// OpFact is one closed-vocabulary operation a summary answers for. An
// unrecognized fact on an older schema reads as unknown, never as false.
type OpFact uint8

const (
	FactAlloc OpFact = iota
	FactRelease
	FactBlock
	FactSuspend
	FactPanic
	FactIO
	FactSync
	FactSpawn
	numOpFacts
)

func (f OpFact) String() string {
	names := [numOpFacts]string{
		FactAlloc: "allocate", FactRelease: "release", FactBlock: "block",
		FactSuspend: "suspend", FactPanic: "panic", FactIO: "perform I/O",
		FactSync: "synchronize", FactSpawn: "spawn a task",
	}
	if int(f) >= len(names) {
		return "unknown operation"
	}
	return names[f]
}

// OpSummarySchemaVersion bumps when the artifact encoding changes shape; an
// unrecognized version reads back as fully unknown, not as "no operations".
const OpSummarySchemaVersion = 1

// OpCause is the first-cause reason behind one fact's Maybe bit: a concrete
// edge, or Unknown for a dynamic target or an untrusted bodiless callee.
type OpCause struct {
	Edge    EffectEdge
	Unknown bool
}

// OpFactState is one fact's answer plus its provenance. InterruptUnsafe only
// applies to FactSync: whether that sync primitive is interrupt-safe.
type OpFactState struct {
	Maybe           bool
	Cause           OpCause
	InterruptUnsafe bool
}

// OpSummary is a node's closed operation summary. Unresolved marks a body
// that still calls a formula variable only a caller's concrete argument can settle; a caller-less strict boundary reads it as unsafe.
type OpSummary struct {
	Known      bool
	Unresolved bool
	Facts      [numOpFacts]OpFactState
}

// opSummary is the public entry point: a fixed node resolves immediately,
// every other one reads the fixpoint inferOpSummaries already settled.
func (r *Result) opSummary(id EntityID) OpSummary {
	if s, ok := r.opFixedSummary(id); ok {
		return s
	}
	return *r.opSummaryPtr(id)
}

func (r *Result) opSummaryPtr(id EntityID) *OpSummary {
	if r.Entities[id].Kind == EntClosure {
		return &r.closure(id).Ops
	}
	return &r.Fn(id).Ops
}

// opFixedSummary is OpSummary's analogue of fixedSummary: a bodiless
// function trusts only its declared `noalloc`, same as isrFixed's rule.
func (r *Result) opFixedSummary(id EntityID) (OpSummary, bool) {
	e := &r.Entities[id]
	if e.Kind == EntClosure {
		return OpSummary{}, false
	}
	if e.Kind != EntFn && e.Kind != EntContract && e.Kind != EntImplicitMain && e.Kind != EntTest {
		return OpSummary{Known: true}, true
	}
	info := r.Fn(id)
	if info.Body != 0 {
		return OpSummary{}, false
	}
	s := OpSummary{Known: true}
	trusted := info.Declared&EffNoalloc != 0
	s.Facts[FactAlloc] = OpFactState{Maybe: !trusted}
	for f := OpFact(0); f < numOpFacts; f++ {
		if f == FactAlloc || trusted {
			continue
		}
		s.Facts[f] = OpFactState{Maybe: true, Cause: OpCause{Unknown: true}}
	}
	return s, true
}
