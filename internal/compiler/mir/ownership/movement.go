package ownership

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/syntax"
)

type MovePosition struct {
	Bid midir.BlockID
	Idx int
}

// MoveEvent is a move, consume or in-place change of root's value; field marks a field move checkCapturedFieldMoves already sees while live.
type MoveEvent struct {
	At    MovePosition
	Root  midir.LocalID
	Node  syntax.NodeID
	Field bool
}
