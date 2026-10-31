package borrow

import (
	midir "kigumi/internal/compiler/mir/ir"
)

func (state *Analyzer) Check(f *midir.Func) {
	analyzeBorrows(state, f)
	checkBorrowedMoves(state, f)
	prior := checkImmediateCaptures(state, f)
	checkErasedReaders(state, f, prior)
}
