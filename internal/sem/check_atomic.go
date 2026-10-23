package sem

import (
	"strings"

	"kigumi/internal/syntax"
)

type MemOp struct {
	Op     string
	Cell   bool
	Atomic bool
	Addr   bool
}

// AtomicOrders lists the AtomicOrder variants in the order Index counts them.
var AtomicOrders = []string{"Relaxed", "Acquire", "Release", "AcqRel", "SeqCst"}

func (r *Result) memOpOf(fn EntityID) (MemOp, bool) {
	e := &r.Entities[fn]
	if e.Kind != EntFn || r.Packages[e.Pkg].Path != "std/mem" {
		return MemOp{}, false
	}
	cell := false
	if owner := r.Fn(fn).Owner; owner != 0 {
		if r.Entities[owner].Name != "UnsafeCell" {
			return MemOp{}, false
		}
		cell = true
	}
	switch name := e.Name; {
	case name == "volatileRead" || name == "volatileWrite":
		return MemOp{Op: name}, !cell
	case name == "volatileReadAt" || name == "volatileWriteAt":
		return MemOp{Op: name, Addr: true}, !cell
	case strings.HasPrefix(name, "atomic"):
		op := name[len("atomic"):]
		return MemOp{Op: strings.ToLower(op[:1]) + op[1:], Cell: cell, Atomic: true}, true
	}
	return MemOp{}, false
}

func (r *Result) MemOpOf(fn EntityID) (MemOp, bool) { return r.memOpOf(fn) }

func (m MemOp) place() bool { return m.Atomic && !m.Cell }

func (m MemOp) arith() bool {
	switch m.Op {
	case "add", "sub", "and", "or", "xor":
		return true
	}
	return false
}

func (c *checker) markAtomicPlace(fn EntityID, args []syntax.NodeID) {
	if op, ok := c.r.memOpOf(fn); !ok || !op.place() || len(args) == 0 || c.t.Kind(args[0]) != syntax.BorrowExpr {
		return
	}
	if c.atomicPlace == nil {
		c.atomicPlace = map[syntax.NodeID]bool{}
	}
	c.atomicPlace[args[0]] = true
}

func (c *checker) memOpCheck(n syntax.NodeID, fn EntityID, inst []TypeID, args []syntax.NodeID, recv TypeID) {
	op, ok := c.r.memOpOf(fn)
	if !ok {
		return
	}
	name := c.r.Entities[fn].Name
	c.memElemCheck(n, op, name, c.memElem(op, inst, recv))
	if !op.Atomic {
		return
	}
	if op.place() {
		c.atomicPlaceCheck(n, name, args)
	}
	if len(args) > 0 {
		c.atomicOrderCheck(args[len(args)-1], op, name)
	}
}

func (c *checker) memElem(op MemOp, inst []TypeID, recv TypeID) TypeID {
	if !op.Cell {
		if len(inst) == 0 {
			return 0
		}
		return c.vars.resolve(inst[0])
	}
	rt := c.vars.resolve(recv)
	if c.r.Types.Kind(rt) == KRef {
		rt = c.vars.resolve(c.r.Types.Node(rt).Elem)
	}
	if rn := c.r.Types.Node(rt); rn.Kind == KNamed && len(rn.Args) > 0 {
		return c.vars.resolve(rn.Args[0])
	}
	return 0
}

func (c *checker) memElemCheck(n syntax.NodeID, op MemOp, name string, t TypeID) {
	tt := c.r.Types
	if t == 0 || t == TyPoison || tt.Kind(t) == KVar {
		return
	}
	switch {
	case op.Addr:
		if t == TyBool || tt.IsInteger(t) && t != TyI128 && t != TyU128 {
			return
		}
		c.errAt(n, cAtomicType, c.r.TypeString(t), name, "T is an integer type of at most 64 bits or Bool")
	case !op.Atomic:
		if c.cValueScalar(t) && !tt.IsCFn(t) {
			return
		}
		c.errAt(n, cAtomicType, c.r.TypeString(t), name, "T is a fixed-width number, Bool, Char or a raw pointer")
	case tt.IsInteger(t) && t != TyI128 && t != TyU128:
	case !op.arith() && t == TyBool:
	case !op.arith() && op.Cell && tt.Kind(t) == KPtr:
	case op.arith():
		c.errAt(n, cAtomicType, c.r.TypeString(t), name, "T is an integer type of at most 64 bits")
	case op.Cell:
		c.errAt(n, cAtomicType, c.r.TypeString(t), name, "T is an integer type of at most 64 bits, Bool or a raw pointer")
	default:
		c.errAt(n, cAtomicType, c.r.TypeString(t), name, "T is an integer type of at most 64 bits or Bool")
	}
}

func (c *checker) atomicPlaceCheck(n syntax.NodeID, name string, args []syntax.NodeID) {
	if len(args) == 0 {
		return
	}
	if c.t.Kind(args[0]) != syntax.BorrowExpr {
		c.errAt(args[0], cAtomicPlace, "the place of `"+name+"` is not `&NAME`")
		return
	}
	ent, ok := c.staticRootOf(c.t.View(args[0]).AsBorrow().Operand().ID())
	if !ok || c.t.Kind(c.stripParens(c.t.View(args[0]).AsBorrow().Operand().ID())) != syntax.Ident {
		c.errAt(args[0], cAtomicPlace, "the place of `"+name+"` is not a bare static")
		return
	}
	if info := c.r.Static(ent); !info.Mut || info.Owner != StaticOwned {
		c.errAt(args[0], cAtomicPlace, "`"+c.r.Entities[ent].Name+"` is not a Kigumi-owned `static mut`")
	}
}

func (c *checker) stripParens(n syntax.NodeID) syntax.NodeID {
	for c.t.Kind(n) == syntax.Paren {
		n = c.t.View(n).AsParen().Inner().ID()
	}
	return n
}

func (c *checker) atomicOrderCheck(arg syntax.NodeID, op MemOp, name string) {
	arg = c.stripParens(arg)
	v := c.info.Uses[arg]
	if v == 0 || c.r.Entities[v].Kind != EntVariant || c.r.Entities[c.r.Entities[v].Parent].Name != "AtomicOrder" || c.r.Packages[c.r.Entities[v].Pkg].Path != "std/mem" {
		c.errAt(arg, cAtomicOrder, "the memory order of `"+name+"` is not a literal `AtomicOrder` variant")
		return
	}
	order := c.r.Entities[v].Name
	bad := op.Op == "load" && (order == "Release" || order == "AcqRel") || op.Op == "store" && (order == "Acquire" || order == "AcqRel")
	if bad {
		c.errAt(arg, cAtomicOrder, "`"+order+"` is not a valid memory order for `"+name+"`")
	}
}

func (c *checker) memValueCheck(v ffiValueCheck) {
	op, ok := c.r.memOpOf(v.fn)
	if !ok {
		return
	}
	name := c.r.Entities[v.fn].Name
	if op.place() || op.Addr {
		c.errAt(v.node, cAtomicPlace, "`"+name+"` is lowered at its call site and cannot be used as a function value")
		return
	}
	c.memElemCheck(v.node, op, name, c.memElem(op, v.inst, v.recv))
}
