// Package mir lowers checked trees to a block-structured IR with mutable
// locals (SSA is left to LLVM's mem2reg), runs the ownership analysis and
// inserts drops.
package mir

import (
	"kigumi/internal/diag"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
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
	OpBoxDirect                     // Dst = a carrier holding the direct aggregate Args[0] (Type: its type)
	OpUnboxDirect                   // Dst = the direct aggregate in carrier Args[0] (Type: its type)
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
	// Own marks a Copy the share analysis adds so a value stops sharing a box with another name.
	Own bool
	// Last marks a Copy of a named local nothing reads again, which a boxed one hands over by move instead.
	Last bool
	// Async marks an OpFnItem/OpClosure for an async fn value: calling it builds a Future instead of running the body.
	Async bool
	// EndScope marks an OpDrop synthesized by scope exit, not a user-written drop;
	// the borrow checker reports it with wording about the scope ending.
	EndScope bool
	// Scoped marks one call's receiver-cell plumbing: an OpNewCell a backend
	// may place in the frame, and the OpDrop that gives its content back.
	Scoped bool
	// Fixed marks an OpBoxDirect of a compile-time value: both backends hand over its one immortal.
	Fixed bool
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

type Local struct {
	Name string
	Type sem.TypeID
	Ent  sem.EntityID
	Cell bool
	// Borrowed marks a `self` / `mut self` receiver: an alias of the
	// caller's value that the method neither owns nor may move out.
	Borrowed bool
	// Cross marks a local PlanDirect made for one crossing: the crossing
	// owns what it holds, the ownership plan having run before it existed.
	Cross bool
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
	// ImplicitOk marks a script's implicit main: falling off its statement
	// list returns a bare Unit standing for Ok(()), unlike every other
	// return of a fallible-Unit function, which must carry a real Ok.
	ImplicitOk bool
	// Async marks a body that runs as a resumable state machine: Suspends
	// counts its points, Slot places a local in the Frame slots (-1: none).
	Async    bool
	Suspends int
	Slot     []int
	Frame    int
	// Cleanup lists, per suspension point, the frame slots an abandoned
	// body releases, innermost binding first.
	Cleanup   [][]LocalID
	PanicDrop []LocalID
	// Direct marks the locals a backend keeps as an unboxed aggregate and
	// DSig does the same for the parameters and the result; PlanDirect sets both.
	Direct []bool
	DSig   *DirectSig
	// unownedCache memoizes unownedRefs for one build-then-verify round; Verify drops it on return.
	unownedCache  []bool
	unownedCached bool
	// rcBase is the local count before plan-rc added its temporaries.
	rcBase int
}

type Program struct {
	R     *sem.Result
	Funcs []*Func
	ByEnt map[sem.EntityID]*Func
	Entry *Func
	// Comptime lists the comptime blocks, each lowered as a function of its
	// own that the driver evaluates and folds before anything runs.
	Comptime []*ComptimeFunc
	// Resources is every function's resource summary, set once
	// ComputeResourceSummaries runs; nil before then.
	Resources map[sem.EntityID]*ResourceSummary
	unwind    unwindMemo
	Diags     []FileDiag
}
