package ir

// Shape is the operand contract of one opcode: argument count (max -1 for any),
// whether it yields a value, and which fields it must carry.
type Shape struct {
	Min, Max int
	Result   bool
	Entity   bool
	Type     bool
	Text     bool
}

var shapes = map[Opcode]Shape{
	OpConst:       {0, 0, true, false, true, false},
	OpUnit:        {0, 0, true, false, false, false},
	OpCopy:        {1, 1, true, false, false, false},
	OpMove:        {1, 1, true, false, false, false},
	OpCall:        {0, -1, true, true, false, false},
	OpCallValue:   {1, -1, true, false, false, false},
	OpBuiltin:     {0, -1, true, false, false, true},
	OpBinary:      {2, 2, true, false, false, true},
	OpUnary:       {1, 1, true, false, false, true},
	OpRecord:      {0, -1, true, true, false, false},
	OpVariant:     {0, -1, true, true, true, false},
	OpField:       {1, 1, true, false, false, false},
	OpFieldMove:   {1, 1, true, false, false, false},
	OpSetField:    {2, 2, false, false, false, false},
	OpArray:       {0, -1, true, false, false, false},
	OpIndex:       {2, 2, true, false, false, false},
	OpSetIndex:    {3, 3, false, false, false, false},
	OpTag:         {1, 1, true, false, false, false},
	OpPayload:     {1, 1, true, false, false, false},
	OpIsVariant:   {1, 1, true, true, false, false},
	OpIsType:      {1, 1, true, true, false, false},
	OpUnbox:       {1, 1, true, false, false, false},
	OpBox:         {1, 1, true, false, true, false},
	OpLendBox:     {1, 1, true, false, true, false},
	OpClosure:     {0, -1, true, true, false, false},
	OpFnItem:      {0, 0, true, true, false, false},
	OpBind:        {0, -1, true, true, false, false},
	OpCFnPtr:      {0, 0, true, true, true, false},
	OpCallC:       {1, -1, true, false, true, false},
	OpAsm:         {0, -1, true, false, true, false},
	OpBorrow:      {1, 1, true, false, false, false},
	OpNewCell:     {1, 1, true, false, false, false},
	OpCellGet:     {1, 1, true, false, false, false},
	OpCellTake:    {1, 1, true, false, false, false},
	OpCellSet:     {2, 2, false, false, false, false},
	OpInterp:      {0, -1, true, false, false, false},
	OpShell:       {0, -1, true, false, false, false},
	OpDrop:        {1, 1, false, false, false, false},
	OpAlias:       {1, 1, true, false, false, false},
	OpShare:       {1, 1, true, false, false, false},
	OpRelease:     {1, 1, false, false, false, false},
	OpPanic:       {0, 0, false, false, false, true},
	OpNop:         {0, 0, false, false, false, false},
	OpGlobalGet:   {0, 0, true, true, false, false},
	OpGlobalSet:   {1, 1, false, true, false, false},
	OpFutureNew:   {1, -1, true, false, false, false},
	OpAwait:       {1, 1, true, false, false, false},
	OpFutureDone:  {1, 1, false, false, false, false},
	OpCancelled:   {0, 0, true, false, false, false},
	OpCancelDone:  {0, 0, false, false, false, false},
	OpBoxDirect:   {1, 1, true, false, true, false},
	OpUnboxDirect: {1, 1, true, false, true, false},
}

var yielding = func() (out [256]bool) {
	for op, s := range shapes {
		out[op] = s.Result
	}
	return out
}()

// Yields reports whether an opcode defines its destination.
func (o Opcode) Yields() bool { return yielding[o] }

func (o Opcode) Shape() (Shape, bool) { s, ok := shapes[o]; return s, ok }
