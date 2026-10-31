package llgen

import (
	"fmt"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

func (e *emitter) directStatic(ent sem.EntityID) (Type, bool) {
	if !e.r.DirectStatic(ent) {
		return TVoid, false
	}
	return e.scalarType(e.r.Entity(ent).Type)
}

func (e *emitter) loadDirectStatic(sb *strings.Builder, ent sem.EntityID, ty Type) Value {
	t := e.newTmp()
	fmt.Fprintf(sb, "  %s = load %s, ptr %s\n", t, ty, e.globalSym(ent))
	return Value{ty, t}
}

func (e *emitter) storeDirectStatic(sb *strings.Builder, ent sem.EntityID, v Value) {
	fmt.Fprintf(sb, "  store %s %s, ptr %s\n", v.Type, v.Text, e.globalSym(ent))
}

// tryDirectStaticInst reads or writes a direct static on registers, so a store allocates no box.
func (e *emitter) tryDirectStaticInst(sb *strings.Builder, f *midir.Func, in midir.Inst) bool {
	ty, ok := e.directStatic(in.Ent)
	if !ok {
		return false
	}
	switch in.Op {
	case midir.OpGlobalGet:
		if !e.isScalar(in.Dst) {
			return false
		}
		e.storeNative(sb, in.Dst, e.loadDirectStatic(sb, in.Ent, ty))
		return true
	case midir.OpGlobalSet:
		v, ok := e.scalarOperand(sb, f, in.Args[0])
		if !ok {
			return false
		}
		e.storeDirectStatic(sb, in.Ent, v)
		if !e.isScalar(in.Args[0]) {
			e.irb(sb).Call(rtRelease, vptr(e.load(sb, in.Args[0])))
		}
		return true
	}
	return false
}
