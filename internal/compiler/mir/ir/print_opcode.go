package ir

var opNames = [...]string{
	OpConst: "const", OpUnit: "unit", OpCopy: "copy", OpMove: "move", OpCall: "call",
	OpCallValue: "callv", OpBuiltin: "builtin", OpBinary: "bin", OpUnary: "un",
	OpRecord: "record", OpVariant: "variant", OpField: "field", OpFieldMove: "fieldmove", OpSetField: "setfield",
	OpArray: "array", OpIndex: "index", OpSetIndex: "setindex", OpTag: "tag",
	OpPayload: "payload", OpIsVariant: "isvariant", OpIsType: "istype", OpUnbox: "unbox",
	OpBox: "box", OpLendBox: "lendbox", OpClosure: "closure", OpFnItem: "fnitem", OpBind: "bind", OpCFnPtr: "cfnptr", OpCallC: "callc", OpAsm: "asm", OpBorrow: "borrow",
	OpNewCell: "newcell", OpCellGet: "cellget", OpCellTake: "celltake", OpCellSet: "cellset", OpInterp: "interp",
	OpShell: "shell", OpDrop: "drop", OpAlias: "alias", OpShare: "share", OpRelease: "release", OpPanic: "panic", OpNop: "nop",
	OpGlobalGet: "globalget", OpGlobalSet: "globalset",
	OpFutureNew: "future", OpAwait: "await", OpFutureDone: "complete",
	OpCancelled: "cancelled", OpCancelDone: "cancel",
	OpBoxDirect: "boxdirect", OpUnboxDirect: "unboxdirect",
}

func (o Opcode) String() string { return opNames[o] }
