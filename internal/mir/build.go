package mir

import (
	"fmt"

	"kigumi/internal/hir"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

// Build lowers every body of a checked module through HIR.
func Build(r *sem.Result) *Program {
	p := &Program{R: r, ByEnt: map[sem.EntityID]*Func{}}
	for id := 1; id < len(r.Entities); id++ {
		// A minted instantiation has no body of its own to lower.
		if _, minted := r.InstanceOf(sem.EntityID(id)); minted {
			continue
		}
		switch r.Entity(sem.EntityID(id)).Kind {
		case sem.EntFn, sem.EntImplicitMain, sem.EntTest:
			if hf := hir.Lower(r, sem.EntityID(id)); hf != nil {
				p.build(hf)
			}
		}
	}
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
	shared map[LocalID]bool
	p      *Program
	r      *sem.Result
	fn     *Func
	cur    BlockID
	locals map[sem.EntityID]LocalID
	scopes []*scope
	loops  []*loopCtx
	ret    sem.TypeID
	// argFixups holds pending scalar &mut write-backs for the call being built.
	argFixups []argFixup
	// unowned tracks which locals hold no count of their own: a borrow, or anything that only forwards one.
	unowned map[LocalID]bool
}

// scope tracks the owned locals and deferred statements of one block.
type scope struct {
	owned  []LocalID
	defers []*hir.Stmt
}

type loopCtx struct {
	cont, exit BlockID
	depth      int
	// valued and result serve a `break` carrying a value (A9-1b): result is
	// the local it assigns before jumping to exit. Local 0 is a valid id,
	// so valued (not a zero check on result) says whether this loop has one.
	valued bool
	result LocalID
}

func (b *builder) newBlock() BlockID {
	b.fn.Blocks = append(b.fn.Blocks, Block{})
	b.cur = BlockID(len(b.fn.Blocks) - 1)
	return b.cur
}

// newBlockAt creates a block without moving the cursor.
func (b *builder) newBlockAt() BlockID {
	saved := b.cur
	id := b.newBlock()
	b.cur = saved
	return id
}

func (b *builder) block() *Block { return &b.fn.Blocks[b.cur] }

func (b *builder) emit(in Inst) LocalID {
	// A reference with no count of its own has nothing to drop; a Scoped
	// drop releases a receiver cell's own reference, not an alias.
	if in.Op == OpDrop && !in.Scoped && len(in.Args) == 1 && (b.unowned[in.Args[0]] || b.fn.Locals[in.Args[0]].Borrowed) {
		return in.Dst
	}
	if b.block().Term.Op != TermNone {
		b.newBlock()
	}
	b.block().Insts = append(b.block().Insts, in)
	b.trackUnowned(in)
	return in.Dst
}

func (b *builder) trackUnowned(in Inst) {
	switch in.Op {
	case OpBorrow:
		b.unowned[in.Dst] = true
	case OpAlias, OpMove:
		if len(in.Args) == 1 {
			b.unowned[in.Dst] = b.unowned[in.Args[0]]
		}
	case OpCopy:
		if len(in.Args) == 1 {
			b.unowned[in.Dst] = !in.Share && b.unowned[in.Args[0]]
		}
	}
}

func (b *builder) term(t Term) {
	if b.block().Term.Op == TermNone {
		b.block().Term = t
	}
}

func (b *builder) jump(to BlockID) { b.term(Term{Op: TermJump, Targets: []BlockID{to}}) }

func (b *builder) temp(t sem.TypeID) LocalID {
	b.fn.Locals = append(b.fn.Locals, Local{Name: fmt.Sprintf("t%d", len(b.fn.Locals)), Type: t})
	return LocalID(len(b.fn.Locals) - 1)
}

func (b *builder) unit(n syntax.NodeID) LocalID {
	return b.emit(Inst{Op: OpUnit, Dst: b.temp(sem.TyUnit), Node: n})
}

// never yields a placeholder local after a diverging expression.
func (b *builder) never(n syntax.NodeID) LocalID {
	b.newBlock()
	return b.emit(Inst{Op: OpUnit, Dst: b.temp(sem.TyNever), Node: n})
}

func (b *builder) assign(dst, src LocalID) {
	if b.fn.Locals[src].Type == sem.TyNever {
		return
	}
	b.emit(Inst{Op: OpCopy, Dst: dst, Args: []LocalID{src}})
}

// returnValue ends the function with v; the caller has unwound scopes.
func (b *builder) returnValue(v LocalID) {
	b.term(Term{Op: TermReturn, Args: []LocalID{v}})
}
