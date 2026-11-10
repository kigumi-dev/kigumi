package build

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/compiler/syntax"
)

// expr lowers an expression to a local holding its value, with the
// checker's coercion steps applied.
func (b *builder) expr(e *hir.Expr) midir.LocalID {
	v := b.exprRaw(e)
	// A fallback lifts only its hit value (build_ops.go); the miss arm is
	// already at the lifted type.
	if e.Kind == hir.Fallback && e.HitLift {
		return v
	}
	if isDefinedView(e) {
		v = b.emit(midir.Inst{Op: midir.OpAlias, Dst: b.temp(e.Type), Args: []midir.LocalID{v}, Node: e.Node})
	}
	return b.coerce(e.Coercion, e.Node, v)
}

func isDefinedView(e *hir.Expr) bool {
	return e.Kind == hir.DefinedWrap || e.Kind == hir.DefinedUnwrap
}

// definedBase skips nested wrap/unwrap layers without a coercion: they share
// one representation, so the outermost layer retags once.
func definedBase(e *hir.Expr) *hir.Expr {
	for isDefinedView(e) && len(e.Coerce) == 0 {
		e = e.Args[0]
	}
	return e
}

func (b *builder) exprRaw(e *hir.Expr) midir.LocalID {
	n, t := e.Node, e.Type
	switch e.Kind {
	case hir.Local:
		id := b.local(e.Ent)
		if b.fn.Locals[id].Cell {
			return b.emit(midir.Inst{Op: midir.OpCellGet, Dst: b.temp(b.r.Entity(e.Ent).Type), Args: []midir.LocalID{id}, Node: n})
		}
		return id
	case hir.Lit:
		return b.emit(midir.Inst{Op: midir.OpConst, Dst: b.temp(t), Lit: e.Lit, Type: t, Node: n})
	case hir.ArgRef:
		v := b.argRef(e.Index)
		if e.Index >= 0 && b.r.IsCopy(t) {
			return b.emit(midir.Inst{Op: midir.OpShare, Dst: b.temp(t), Args: []midir.LocalID{v}, Copy: true, Node: n})
		}
		return v
	case hir.Global:
		return b.emit(midir.Inst{Op: midir.OpGlobalGet, Dst: b.temp(t), Ent: e.Ent, Node: n})
	case hir.FnItem:
		async := b.r.Fn(e.Ent).Declared&sem.EffAsync != 0
		if len(e.Passes) > 0 {
			return b.emit(midir.Inst{Op: midir.OpBind, Dst: b.temp(t), Ent: e.Ent, Args: b.witnessArgs(e.Passes, n), Async: async, Node: n})
		}
		return b.emit(midir.Inst{Op: midir.OpFnItem, Dst: b.temp(t), Ent: e.Ent, Async: async, Node: n})
	case hir.Host:
		return b.emit(midir.Inst{Op: midir.OpBuiltin, Dst: b.temp(t), Str: "host", Node: n})
	case hir.Wrap:
		return b.expr(e.Args[0])
	case hir.DefinedWrap, hir.DefinedUnwrap:
		return b.expr(definedBase(e.Args[0]))
	case hir.Unary:
		v := b.expr(e.Args[0])
		if e.Ent != 0 {
			return b.emit(midir.Inst{Op: midir.OpCall, Dst: b.temp(t), Ent: e.Ent, Args: []midir.LocalID{v}, Node: n})
		}
		return b.emit(midir.Inst{Op: midir.OpUnary, Dst: b.temp(t), Str: e.Name, Args: []midir.LocalID{v}, Node: n})
	case hir.Binary:
		l := b.snapshot(b.expr(e.Args[0]), e.Args[0], e.Args[0].Node, e.Args[1])
		r := b.expr(e.Args[1])
		if e.Ent != 0 {
			return b.emit(midir.Inst{Op: midir.OpCall, Dst: b.temp(t), Ent: e.Ent, Args: []midir.LocalID{l, r}, Node: n})
		}
		return b.emit(midir.Inst{Op: midir.OpBinary, Dst: b.temp(t), Str: e.Name, Args: []midir.LocalID{l, r}, Node: n})
	case hir.And:
		return b.valueAnd(e)
	case hir.Fallback:
		return b.fallback(e)
	case hir.Pipe:
		return b.pipe(e)
	case hir.Is:
		v := b.expr(e.Args[0])
		return b.isExpr(e.Pat, v, b.borrowedIsTemp(e, v))
	case hir.Call, hir.Intrinsic, hir.Variant, hir.MethodCall, hir.StaticCall, hir.ValueCall:
		return b.call(e)
	case hir.Index:
		base := b.expr(e.Args[0])
		idx := b.expr(e.Args[1])
		return b.emit(midir.Inst{Op: midir.OpIndex, Dst: b.temp(t), Args: []midir.LocalID{base, idx}, Node: n})
	case hir.Field:
		base := b.expr(e.Args[0])
		return b.emit(midir.Inst{Op: midir.OpField, Dst: b.temp(t), Args: []midir.LocalID{base}, Index: e.Index, Node: n})
	case hir.FieldMove:
		base := b.expr(e.Args[0])
		if b.r.Types.RefMut(t) {
			return b.refFieldMove(base, t, e.Index, n)
		}
		return b.emit(midir.Inst{Op: midir.OpFieldMove, Dst: b.temp(t), Args: []midir.LocalID{base}, Index: e.Index, Node: n})
	case hir.MethodValue:
		return b.methodValue(e, b.methodReceiver(e, e.Type), t)
	case hir.OptField:
		return b.optField(e)
	case hir.Try:
		return b.try(e)
	case hir.Record:
		return b.recordLit(e)
	case hir.Lambda:
		return b.lambda(e)
	case hir.BlockExpr:
		return b.blockExpr(e.Block)
	case hir.If:
		return b.ifExpr(e)
	case hir.IfLet:
		return b.ifLet(e)
	case hir.Match:
		return b.matchExpr(e)
	case hir.Loop:
		return b.forExpr(e)
	case hir.Return:
		b.markExit(0)
		var v midir.LocalID
		if len(e.Args) == 0 {
			v = b.unit(n)
			// A checker coercion has no value node to attach to here, so a
			// bare `return` in a fallible-Void function is Ok-wrapped
			// directly against the function's own return type.
			if val, _, ok := b.r.Types.IsResult(b.ret); ok && val == sem.TyUnit {
				v = b.emit(midir.Inst{Op: midir.OpVariant, Dst: b.temp(b.ret), Ent: b.variantNamed(b.r.Types.ResultEnt(), "Ok"), Type: b.ret, Args: []midir.LocalID{v}, Node: n})
			}
			b.returnUnwinding(v, nil, n)
		} else {
			v = b.fullValue(func() midir.LocalID { return b.exprMoved(e.Args[0]) })
			b.returnUnwinding(v, e.Args[0], n)
		}
		return b.never(n)
	case hir.Fail:
		// The error leaves every scope, so a named local travels in a
		// temporary before the scopes drop it.
		v := b.fullValue(func() midir.LocalID { return b.exprMoved(e.Args[0]) })
		b.failWith(n, v)
		return b.never(n)
	case hir.Break, hir.Continue:
		l := b.loops[len(b.loops)-1]
		b.markExit(l.depth)
		var v midir.LocalID
		if e.Kind == hir.Break && len(e.Args) > 0 {
			v = b.exprMoved(e.Args[0])
		}
		b.unwindTo(l.depth, false)
		if e.Kind == hir.Break {
			if l.valued {
				b.assign(l.result, v)
			}
			b.jump(l.exit)
		} else {
			b.jump(l.cont)
		}
		return b.never(n)
	case hir.Asm:
		return b.asm(e)
	case hir.Await:
		return b.await(e, n, t, "")
	case hir.Allocator:
		// The guard pops the allocator when it is dropped, so every exit
		// from the block, structured or not, restores the previous one.
		scope := b.expr(e.Args[0])
		b.pushScope()
		b.own(b.emit(midir.Inst{Op: midir.OpBuiltin, Dst: b.temp(e.Args[0].Type), Str: "alloc.push", Args: []midir.LocalID{scope}, Node: n}))
		result := b.detach(b.blockExpr(e.Block))
		b.exitScope()
		return result
	case hir.Borrow:
		return b.borrowExpr(e, n, t)
	case hir.Interp:
		// The result temporary is numbered before the pieces it interpolates.
		in := midir.Inst{Op: midir.OpInterp, Dst: b.temp(sem.TyString), Strs: e.Strs, Node: n}
		for i, a := range e.Args {
			in.Args = append(in.Args, b.snapshot(b.expr(a), a, a.Node, e.Args[i+1:]...))
		}
		return b.emit(in)
	case hir.Shell:
		in := midir.Inst{Op: midir.OpShell, Dst: b.temp(t), Strs: e.Strs, Node: n}
		for i, a := range e.Args {
			v := b.snapshot(b.expr(a), a, a.Node, e.Args[i+1:]...)
			in.Args = append(in.Args, b.shellArgText(v, a.Type, a.Node))
		}
		return b.emit(in)
	case hir.Comptime:
		return b.comptime(e)
	case hir.Unit:
		return b.unit(n)
	}
	return b.emit(midir.Inst{Op: midir.OpPanic, Dst: b.temp(t), Str: e.Name, Node: n})
}

// shellArgText converts an interpolation whose type has a standard text form
// to that text, so both engines see a String word like any other. §10.7 lists
// `fs.Path` and `os.OsString` beside String and integers; the checker accepts
// them and this call is their implementation. A plan word is a String, so an
// OsString that is not valid UTF-8 keeps its display text (U+FFFD for the
// undecodable bytes), the same lossy conversion `debug` shows.
func (b *builder) shellArgText(v midir.LocalID, t sem.TypeID, n syntax.NodeID) midir.LocalID {
	method := b.shellArgTextMethod(t)
	if method == 0 {
		return v
	}
	return b.emit(midir.Inst{Op: midir.OpCall, Dst: b.temp(sem.TyString), Ent: method, Args: []midir.LocalID{v}, Node: n})
}

// shellArgTextMethod is the zero-argument text method of a shell argument
// type, or 0 when the type has none and lowers as it stands.
func (b *builder) shellArgTextMethod(t sem.TypeID) sem.EntityID {
	tt := b.r.Types
	if tt.Kind(t) != sem.KNamed {
		return 0
	}
	ent := tt.Node(t).Ent
	if ent == 0 {
		return 0
	}
	e := b.r.Entity(ent)
	pkg := b.r.Packages[e.Pkg].Path
	name := ""
	switch {
	case pkg == "std/fs" && e.Name == "Path":
		name = "toString"
	case pkg == "std/os" && e.Name == "OsString":
		name = "toStringLossy"
	default:
		return 0
	}
	set, ok := b.r.TypeDecl(ent).Members[name]
	if !ok || int(set) >= len(b.r.Overloads) || len(b.r.Overloads[set].Members) == 0 {
		return 0
	}
	return b.r.Overloads[set].Members[0]
}
