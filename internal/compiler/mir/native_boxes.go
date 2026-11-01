package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// ArgBox is a call argument whose box the native lowering makes in the caller, at the call or where the value is made.
type ArgBox struct {
	Block midir.BlockID
	Inst  int
	Arg   int
}

type ResultBox struct {
	Block midir.BlockID
	Inst  int
}

type OperationBox struct {
	Block midir.BlockID
	Inst  int
	Type  sem.TypeID
}

type NativeBoxSites struct {
	Arguments  map[*midir.Func][]ArgBox
	Results    map[*midir.Func][]ResultBox
	Operations map[*midir.Func][]OperationBox
}
