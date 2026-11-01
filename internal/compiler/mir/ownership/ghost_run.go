package ownership

import midir "kigumi/internal/compiler/mir/ir"

type MarkPos struct {
	Blk midir.BlockID
	Idx int
}

// GhostRun is a lowered defer(fail) no failing exit runs: blocks [Entry, End) hold its body, reachable only through the analysis.
type GhostRun struct {
	Entry, End midir.BlockID
	Marks      []MarkPos
}
