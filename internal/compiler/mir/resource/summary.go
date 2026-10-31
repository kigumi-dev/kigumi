package resource

// BoundKind is a resource bound's three-state answer: a proven byte
// ceiling, no finite ceiling (recursion), or not enough evidence to tell.
type BoundKind uint8

const (
	Bounded BoundKind = iota
	Unbounded
	Unknown
)

func (k BoundKind) String() string {
	switch k {
	case Bounded:
		return "bounded"
	case Unbounded:
		return "unbounded"
	default:
		return "unknown"
	}
}

// StackBound is one function's call-chain stack bound from that entry:
// Bytes when Bounded, Reason/Path (names to the first cause) otherwise.
type StackBound struct {
	Kind   BoundKind
	Bytes  uint64
	Reason string
	Path   []string
}

// Summary holds one function's resource bounds; Async is nil for a
// non-async function, and Static repeats the program's static-mut total (module-scoped).
type Summary struct {
	FrameBytes uint64
	Chain      StackBound
	Async      *StackBound
	Static     uint64
}
