package llgen

import (
	"strings"

	"kigumi/internal/compiler/mir/instance"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// siteNode is the runtime type a box, type test, downcast or record of type t names: the one in.Rt composed, or
// t's static node.
func (e *emitter) siteNode(sb *strings.Builder, in midir.Inst, t sem.TypeID) string {
	if len(in.Rt) > 0 {
		return e.load(sb, in.Rt[0])
	}
	if !instance.Concrete(e.r, t) {
		e.irb(sb).Call(rtPanic, vptr(e.cstr("internal error: a runtime type names parameters with no environment")))
		return "null"
	}
	return e.tnLoad(sb, t)
}

// typeTest is a box's type test: any instantiation of the entity by its descriptor, one runtime type by its node.
func (e *emitter) typeTest(sb *strings.Builder, in midir.Inst, v string) string {
	if instance.TypeTest(e.r, in.Type) == instance.TestAny {
		return e.irb(sb).Call(rtIstypeAny, vptr(v), vptr(e.desc(in.Ent))).Text
	}
	return e.irb(sb).Call(rtIstypeTn, vptr(v), vptr(e.siteNode(sb, in, in.Type))).Text
}

func (e *emitter) boxSite(sb *strings.Builder, f *midir.Func, in midir.Inst, v string) string {
	node := e.siteNode(sb, in, in.Type)
	vt := e.vtable(in.Type, f.Locals[in.Dst].Type, e.r.Entity(f.Ent).Pkg)
	return e.irb(sb).Call(rtBox, vptr(v), vptr(node), vptr(vt)).Text
}

// recordDesc is the descriptor a record of type t carries: the one made for its runtime type when its drop
// reads that, else its entity's.
func (e *emitter) recordDesc(sb *strings.Builder, in midir.Inst, ent sem.EntityID, t sem.TypeID) string {
	if !e.r.DropReadsInstance(ent) {
		return e.desc(ent)
	}
	return e.irb(sb).Call(rtTnDesc, vptr(e.siteNode(sb, in, t))).Text
}

func (e *emitter) recordDescOf(sb *strings.Builder, t sem.TypeID) string {
	if n := e.r.Types.Node(t); n.Kind == sem.KNamed {
		return e.recordDesc(sb, midir.Inst{}, n.Ent, t)
	}
	return e.descOfType(t)
}

// errorAsCall walks the Context chain from arg for a box of the runtime type node.
func (e *emitter) errorAsCall(sb *strings.Builder, arg, node string) string {
	ctxEnt := e.r.LangItem("Context")
	ctxDesc, wrapped := "null", 0
	if ctxEnt != 0 {
		ctxDesc = e.desc(ctxEnt)
		wrapped = e.r.Field(e.r.FindField(ctxEnt, "wrapped")).Index
	}
	return e.irb(sb).Call(rtErrorAsTn, vptr(arg), vptr(node), vptr(ctxDesc), vi64(wrapped)).Text
}

// typeBuiltin lowers the runtime-type builtins but type.of, which typeOfInst lowers.
func (e *emitter) typeBuiltin(sb *strings.Builder, in midir.Inst, args []string) string {
	switch in.Str {
	case "type.param":
		return e.irb(sb).Call(rtTnArg, vptr(args[0]), vi64(in.Index)).Text
	case "type.arg":
		return e.irb(sb).Call(rtTnBoxarg, vptr(args[0]), vptr(e.desc(in.Ent)), vi64(in.Index)).Text
	}
	node := e.irb(sb).Call(rtTnBoxarg, vptr(args[0]), vptr(e.desc(in.Ent)), vi64(in.Index)).Text
	return e.irb(sb).Call(rtTnValue, vptr(node), vi32(e.constNk(e.r.TypeParam(in.Holes[0]).ConstType))).Text
}

// constHole reports whether argument i of in is an opened const's value a type.of reads, which may sit in a
// register.
func (e *emitter) constHole(f *midir.Func, in midir.Inst, i int) bool {
	return in.Op == midir.OpBuiltin && in.Str == "type.of" && !e.r.Types.IsRtType(f.Locals[in.Args[i]].Type)
}

// typeOfInst lowers a type.of, reading an opened const in a register without boxing it.
func (e *emitter) typeOfInst(sb *strings.Builder, f *midir.Func, in midir.Inst) bool {
	if in.Op != midir.OpBuiltin || in.Str != "type.of" {
		return false
	}
	holes := map[sem.EntityID]tnHole{}
	for i, p := range in.Holes {
		a := in.Args[i]
		switch {
		case e.r.Types.IsRtType(f.Locals[a].Type):
			holes[p] = tnHole{val: e.load(sb, a)}
		case e.isScalar(a):
			holes[p] = tnHole{val: e.scalarConstNode(sb, a, e.r.TypeParam(p).ConstType)}
		default:
			holes[p] = tnHole{val: e.load(sb, a), value: true}
		}
	}
	e.store(sb, in.Dst, e.compose(sb, in.Type, holes))
	return true
}

// scalarConstNode is the node of the const in register a, of type ty, sign-extended to 128 bits as ty says.
func (e *emitter) scalarConstNode(sb *strings.Builder, a midir.LocalID, ty sem.TypeID) string {
	v := e.loadNative(sb, a)
	lo, hi := v.Text, "0"
	switch v.Type {
	case TI1:
		lo = e.newTmp()
		sb.WriteString("  " + lo + " = zext i1 " + v.Text + " to i64\n")
	case TI128:
		l, h := e.splitWide(sb, v.Text)
		lo, hi = l.Text, h.Text
	default:
		if e.r.Types.IsSigned(ty) {
			hi = e.newTmp()
			sb.WriteString("  " + hi + " = ashr i64 " + v.Text + ", 63\n")
		}
	}
	return e.irb(sb).Call(rtTnConstWords, vi64(int(e.constCtor(ty))), vi64text(lo), vi64text(hi)).Text
}

// errorAsValue names the adapter of a std/error.as value built where its target is a runtime type: the value
// captures the target's node.
func (e *emitter) errorAsValue() string {
	sym := "@\"std/error.as$node\""
	if e.cio[sym] {
		return sym
	}
	e.cio[sym] = true
	var sb strings.Builder
	sb.WriteString("define ptr " + sym + "(ptr %env, ptr %args) {\n")
	node := e.irb(&sb).Call(rtIterAt, vptr("%env"), vi64(0), vi32(1)).Text
	arg := e.irb(&sb).Call(rtAt, vptr("%args"), vi64(0)).Text
	sb.WriteString("  ret ptr " + e.errorAsCall(&sb, arg, node) + "\n}\n\n")
	e.helpers.WriteString(sb.String())
	return sym
}
