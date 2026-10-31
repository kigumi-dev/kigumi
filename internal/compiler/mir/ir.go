// Package mir lowers checked trees to a block-structured IR with mutable
// locals (SSA is left to LLVM's mem2reg), runs the ownership analysis and
// inserts drops.
package mir

import (
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
	"kigumi/internal/lang/diag"
)

type LocalID uint32
type BlockID uint32

type Opcode uint8

const (
	OpConst       Opcode = iota + 1 // Dst = Lit (Type)
	OpUnit                          // Dst = ()
	OpCopy                          // Dst = Args[0]
	OpMove                          // Dst = Args[0], source consumed
	OpCall                          // Dst = Ent(Args...)
	OpCallValue                     // Dst = Args[0](Args[1:]...)
	OpBuiltin                       // Dst = Str(Args...) runtime function
	OpBinary                        // Dst = Args[0] Str Args[1]
	OpUnary                         // Dst = Str Args[0]
	OpRecord                        // Dst = Ent { Args... }
	OpVariant                       // Dst = Ent(Args...)
	OpField                         // Dst = Args[0].Index
	OpFieldMove                     // Dst = Args[0].Index, then Args[0].Index is cleared
	OpSetField                      // Args[0].Index = Args[1]
	OpArray                         // Dst = [Args...]
	OpIndex                         // Dst = Args[0][Args[1]]
	OpSetIndex                      // Args[0][Args[1]] = Args[2]
	OpTag                           // Dst = variant tag of Args[0]
	OpPayload                       // Dst = payload Index of Args[0]
	OpIsVariant                     // Dst = Args[0] is variant Ent
	OpIsType                        // Dst = existential Args[0] holds Ent
	OpUnbox                         // Dst = value inside existential Args[0]
	OpBox                           // Dst = existential over Args[0] (Type = dynamic type)
	OpClosure                       // Dst = closure Ent capturing Args...
	OpFnItem                        // Dst = function Ent
	OpBind                          // Dst = function Ent with its trailing witness parameters bound to Args
	OpCFnPtr                        // Dst = C function pointer to Ent
	OpCallC                         // Dst = Args[0](Args[1:]...) through a C function pointer of Type
	OpAsm                           // Dst = inline asm at Node over input Args (sem.AsmInfo)
	OpBorrow                        // Dst = &Args[0] (Index: 1 for mut)
	OpNewCell                       // Dst = cell holding Args[0]
	OpCellGet                       // Dst = *Args[0]
	OpCellTake                      // Dst = *Args[0], handed over: the cell is left empty
	OpCellSet                       // *Args[0] = Args[1]
	OpInterp                        // Dst = string of Args... (Strs holds text pieces)
	OpShell                         // Dst = plan (Strs/Args describe words)
	OpDrop                          // drop Args[0]
	OpAlias                         // Dst = Args[0], the same reference: no runtime call
	OpShare                         // Dst = a new reference to Args[0] (Copy: rt_copy vs rt_retain)
	OpRelease                       // release the temporary Args[0]
	OpPanic                         // abort with Str
	OpNop                           // removed instruction
	OpGlobalGet                     // Dst = value of static Ent
	OpGlobalSet                     // static Ent = Args[0]
	OpFutureNew                     // Dst = Future running async callee Args[0] on Args[1:]
	OpAwait                         // Dst = result of Args[0]; Index is the suspension point
	OpFutureDone                    // Args[0] completes the enclosing async body
	OpCancelled                     // Dst = whether the preceding await was cancelled
	OpCancelDone                    // the async body finishes as cancelled
	OpBoxDirect                     // Dst = a carrier holding the direct aggregate Args[0] (Type: its type)
	OpUnboxDirect                   // Dst = the direct aggregate in carrier Args[0] (Type: its type)
	OpLendBox                       // Dst = `&Interface` view over the borrow Args[0], owning nothing (Type = dynamic type)
)

// Inst is one instruction; the fields used depend on Op.
type Inst struct {
	Op    Opcode
	Dst   LocalID
	Args  []LocalID
	Type  sem.TypeID
	Ent   sem.EntityID
	Str   string
	Strs  []string
	Lit   sem.Literal
	Index int
	Node  syntax.NodeID
	// Share makes an OpCopy take its own reference: the source is a temporary some scope still drops.
	Share bool
	// Copy says an OpShare takes its reference via rt_copy (a Copy type) rather than rt_retain.
	Copy bool
	// IfWritable narrows Copy to a value a write could change, in its box or one inside it; any other is retained.
	IfWritable bool
	// Guarded marks a guarded arm's binding: on an OpCopy the binding does not move its source, on an OpNop at the start of the arm's body Args[0] is the place it takes, moved only once the guard passed (ownership analysis clears the arguments).
	Guarded bool
	// Place marks an OpPayload of `?` or `||` as a place of its operand, as a pattern's payload is.
	Place bool
	// Own marks a Copy the share analysis adds so a value stops sharing a box with another name.
	Own bool
	// Last marks a Copy of a named local nothing reads again, which a boxed one hands over by move instead.
	Last   bool
	Retain bool
	// Async marks an OpFnItem/OpClosure for an async fn value: calling it builds a Future instead of running the body.
	Async bool
	// EndScope marks an OpDrop synthesized by scope exit, not a user-written drop;
	// the borrow checker reports it with wording about the scope ending.
	EndScope bool
	// Lent marks an OpNewCell over a borrowed reference, or an OpUnboxDirect out of one: neither holds a count of what it reads.
	Lent bool
	// Scoped marks one call's receiver-cell plumbing: an OpNewCell a backend
	// may place in the frame, and the OpDrop that gives its content back.
	Scoped bool
	// Fixed marks an OpBoxDirect of a compile-time value: both backends hand over its one immortal.
	Fixed bool
	// NoTrap marks a checked op or panic that cannot fire: a loop counter step or fixed-array index under its guard, a match's fall-through.
	NoTrap bool
	// Rest marks the OpMove and OpDrop that empty a scrutinee a pattern moved a resource out of.
	Rest bool
	// Emptied marks an OpDrop of a record whose every non-Copy field was moved out on every path.
	Emptied bool
	// FullEnd numbers the full expression this instruction ends (see Local.FullExpr).
	FullEnd int32
	// GiveArgs marks the value-call arguments a site hands over at a type-parameter slot, declared to the call (see rt_callv_m).
	GiveArgs uint32
	// LendArgs marks the value-call arguments a site only lends, declared to the call the same way.
	LendArgs     uint32
	argsDeclared bool
	// Prim/Prov are zero unless Ent is a registry-covered std call.
	Prim  PrimitiveId
	Prov  ProviderOpId
	DPrim DirectPrim
}

type TermOp uint8

const (
	TermNone TermOp = iota
	TermJump
	TermBranch // Args[0] ? Targets[0] : Targets[1]
	TermReturn // Args[0]
	TermUnreachable
)

type Term struct {
	Op      TermOp
	Args    []LocalID
	Targets []BlockID
}

type Block struct {
	Insts []Inst
	Term  Term
}

// Func is one lowered body: a user function, a closure, an implicit main
// or a test.
type Func struct {
	Name   string
	Ent    sem.EntityID
	File   sem.FileID
	Params []LocalID
	Ret    sem.TypeID
	Locals []Local
	Blocks []Block
	Diags  []diag.Diagnostic
	// Cleanups lists the defer / defer(fail) statements lowered into the body, so a diagnostic can tell a cleanup's own reads apart.
	Cleanups []syntax.NodeID
	ghosts   ghostSet
	// ImplicitOk marks a script's implicit main: falling off its statement
	// list returns a bare Void standing for Ok(()), unlike every other
	// return of a fallible-Void function, which must carry a real Ok.
	ImplicitOk bool
	// Async marks a body that runs as a resumable state machine: Suspends
	// counts its points, Slot places a local in the Frame slots (-1: none), ViewSlot a lendbox's view.
	Async    bool
	Suspends int
	Slot     []int
	ViewSlot map[LocalID]int
	Frame    int
	// Cleanup lists, per suspension point, the frame slots an abandoned
	// body releases, innermost binding first.
	Cleanup   [][]LocalID
	PanicDrop []LocalID
	// Direct marks the locals a backend keeps as an unboxed aggregate and
	// DSig does the same for the parameters and the result; PlanDirect sets both.
	Direct []bool
	DSig   *DirectSig
	// rcBase is the local count before plan-rc added its temporaries.
	rcBase int
	// Proto is "equals" or "hash" for a body specialization replaced by the direct comparison or hash.
	Proto string
	// Entry is the all-boxed body a function value, method table or adapter runs; Wraps points back.
	Entry *Func
	Wraps *Func
}
