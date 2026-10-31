package mir

import (
	"fmt"

	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// Build lowers what the program can run: every body of the module being
// checked, its comptime blocks, and the std and dependency bodies they reach.
func Build(r *sem.Result) *Program { return build(r, false) }

// BuildAll lowers every body, reachable or not.
func BuildAll(r *sem.Result) *Program { return build(r, true) }

// BuildFor is Build that lowers up front only the own-module bodies of the packages keep accepts.
func BuildFor(r *sem.Result, keep func(path string) bool) *Program {
	return buildFor(r, false, keep)
}

func build(r *sem.Result, all bool) *Program { return buildFor(r, all, nil) }

func buildFor(r *sem.Result, all bool, keep func(path string) bool) *Program {
	p := &Program{R: r, ByEnt: map[sem.EntityID]*midir.Func{}}
	l := &lowering{Program: p, rootKeep: keep}
	l.lowerReachable(all)
	newBorrowAnalysis(p, p.bodyByEnt).checkRefScopes()
	p.Entry = p.ByEnt[r.MainFn()]
	resolveStdOps(p)
	for _, f := range p.allFuncs() {
		if f.Async {
			classifyMobility(f, r)
		}
	}
	refineMobility(p)
	checkHandoff(p)
	return p
}

// builder holds the state of one function under construction.
type builder struct {
	shared map[midir.LocalID]bool
	// argFrames are the argument lists of the calls being evaluated, innermost last, for hir.ArgRef.
	argFrames []argFrame
	// guardedArm is set while the bindings of an arm with a guard are declared; guardMoves collects the places they take by value.
	guardedArm bool
	guardMoves []guardMove
	// condIs holds the patterns of the `is` tests a condition's `&&` chain needs; isMoves holds the places their bindings take, moved on the true branch only.
	condIs  map[*hir.Pat]bool
	isMoves []guardMove
	valueIs map[*hir.Pat]bool
	// valueChain is the value-position `&&` chain whose scope is open.
	valueChain *hir.Expr
	// shortRHS counts the `&&` right operands being lowered, which may never run.
	shortRHS int
	p        *lowering
	r        *sem.Result
	fn       *midir.Func
	cur      midir.BlockID
	locals   map[sem.EntityID]midir.LocalID
	scopes   []*scope
	loops    []*loopCtx
	ret      sem.TypeID
	// argFixups holds pending scalar &mut write-backs for the call being built.
	argFixups []argFixup
	cancelled *cancelPath
	// unowned tracks which locals hold no count of their own: a borrow, or anything that only forwards one.
	unowned   map[midir.LocalID]bool
	joins     map[midir.LocalID][]joinArm
	body      *hir.Block
	ghostRuns ghostSet
	ghosts    []pendingGhost
	full      fullState
}

// scope tracks the owned locals and deferred statements of one block.
type scope struct {
	owned  []midir.LocalID
	defers []*hir.Stmt
	// lowered marks the defer(fail) statements some failing exit emitted.
	lowered map[*hir.Stmt]bool
	marks   []exitMark
}

type loopCtx struct {
	cont, exit midir.BlockID
	depth      int
	// valued and result serve a `break` carrying a value (A9-1b): result is
	// the local it assigns before jumping to exit. Local 0 is a valid id,
	// so valued (not a zero check on result) says whether this loop has one.
	valued bool
	result midir.LocalID
}

func (b *builder) newBlock() midir.BlockID {
	b.fn.Blocks = append(b.fn.Blocks, midir.Block{})
	b.cur = midir.BlockID(len(b.fn.Blocks) - 1)
	return b.cur
}

// newBlockAt creates a block without moving the cursor.
func (b *builder) newBlockAt() midir.BlockID {
	saved := b.cur
	id := b.newBlock()
	b.cur = saved
	return id
}

func (b *builder) block() *midir.Block { return &b.fn.Blocks[b.cur] }

func (b *builder) emit(in midir.Inst) midir.LocalID {
	// A reference with no count of its own has nothing to drop; a Scoped
	// drop releases a receiver cell's own reference, not an alias.
	if in.Op == midir.OpDrop && !in.Scoped && len(in.Args) == 1 && (b.unowned[in.Args[0]] || b.fn.Locals[in.Args[0]].Borrowed) {
		return in.Dst
	}
	if b.block().Term.Op != midir.TermNone {
		b.newBlock()
	}
	b.block().Insts = append(b.block().Insts, in)
	b.trackUnowned(in)
	return in.Dst
}

func (b *builder) trackUnowned(in midir.Inst) {
	switch in.Op {
	case midir.OpBorrow, midir.OpLendBox:
		b.unowned[in.Dst] = true
	case midir.OpAlias, midir.OpMove:
		if len(in.Args) == 1 {
			b.unowned[in.Dst] = b.unowned[in.Args[0]]
		}
	case midir.OpCopy:
		if len(in.Args) == 1 {
			b.unowned[in.Dst] = !in.Share && b.unowned[in.Args[0]]
		}
	}
}

func (b *builder) term(t midir.Term) {
	if b.block().Term.Op == midir.TermNone {
		b.block().Term = t
	}
}

func (b *builder) jump(to midir.BlockID) {
	b.term(midir.Term{Op: midir.TermJump, Targets: []midir.BlockID{to}})
}

func (b *builder) temp(t sem.TypeID) midir.LocalID {
	b.fn.Locals = append(b.fn.Locals, midir.Local{Name: fmt.Sprintf("t%d", len(b.fn.Locals)), Type: t, FullExpr: b.full.id})
	b.full.note(b.r, t)
	return midir.LocalID(len(b.fn.Locals) - 1)
}

func (b *builder) unit(n syntax.NodeID) midir.LocalID {
	return b.emit(midir.Inst{Op: midir.OpUnit, Dst: b.temp(sem.TyUnit), Node: n})
}

// never yields a placeholder local after a diverging expression.
func (b *builder) never(n syntax.NodeID) midir.LocalID {
	b.newBlock()
	return b.emit(midir.Inst{Op: midir.OpUnit, Dst: b.temp(sem.TyNever), Node: n})
}

func (b *builder) assign(dst, src midir.LocalID) {
	if b.fn.Locals[src].Type == sem.TyNever {
		return
	}
	b.emit(midir.Inst{Op: midir.OpCopy, Dst: dst, Args: []midir.LocalID{src}})
	b.noteJoinArm(dst)
}

// returnValue ends the function with v; the caller has unwound scopes.
func (b *builder) returnValue(v midir.LocalID) {
	b.term(midir.Term{Op: midir.TermReturn, Args: []midir.LocalID{v}})
}
