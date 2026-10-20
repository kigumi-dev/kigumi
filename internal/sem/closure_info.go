package sem

type CaptureMode uint8

const (
	capCopy CaptureMode = iota
	capRetain
	capMove
	capCell
	// capBorrow lends a move-only local to a lambda passed straight to a
	// call whose target parameter's escape mode is verified non-escaping.
	capBorrow
)

type Capture struct {
	Local EntityID
	Mode  CaptureMode
}

type ClosureInfo struct {
	Params   []EntityID
	Ret      TypeID
	Sig      TypeID
	Captures []Capture
	Summary  EffectSummary
	Ops      OpSummary
	Edges    []EffectEdge
	Copy     bool
	// Once names a captured place the body consumes, so the closure can be called only once.
	Once string
	// Immediate marks a lambda written as a call argument: its borrowed
	// captures live only for that call.
	Immediate bool
	// Creator held this closure's edges, for a late construction alloc.
	Creator EntityID
	// Scoped mirrors c.allocDepth > 0 at the closure's creation site.
	Scoped bool
	// Method is set on a method value that owns its receiver: no body, one move capture.
	Method EntityID
}

func (c *ClosureInfo) IsMethodValue() bool { return c.Method != 0 }
