package llgen

import (
	"fmt"
	"math"
	"math/big"
	"strings"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

// globalSym names a Kigumi-owned static's internal LLVM storage, mangled
// by entity id since two packages may declare a static of the same name.
func (e *emitter) globalSym(ent sem.EntityID) string {
	return fmt.Sprintf("@\"kg.static.%d\"", uint32(ent))
}

// staticGlobals emits every static's LLVM storage plus kg_init_statics; extern/export statics get their own real C-layout global, owned ones stay the ordinary boxed-ptr representation every other Kigumi value uses.
func (e *emitter) staticGlobals() (decls, initFn string) {
	var g, init strings.Builder
	init.WriteString("define void @kg_init_statics() {\n")
	for _, ent := range e.r.StaticEntities() {
		owner, _ := e.r.StaticOwnerOf(ent)
		typ := e.r.Entity(ent).Type
		switch owner {
		case sem.StaticExtern:
			fmt.Fprintf(&g, "@%s = external global %s\n", e.r.StaticSymbolOf(ent), e.cType(typ))
		case sem.StaticExport:
			lit, _ := e.r.StaticValueOf(ent)
			fmt.Fprintf(&g, "@%s = global %s %s%s\n", e.r.StaticSymbolOf(ent), e.cType(typ), cLiteralText(lit, e.cType(typ)), sectionClause(e.r.StaticSectionOf(ent)))
		default:
			fmt.Fprintf(&g, "%s = internal global ptr null%s\n", e.globalSym(ent), sectionClause(e.r.StaticSectionOf(ent)))
			lit, _ := e.r.StaticValueOf(ent)
			v := e.constant(&init, mir.Inst{Lit: lit, Type: typ})
			fmt.Fprintf(&init, "  store ptr %s, ptr %s\n", v, e.globalSym(ent))
		}
	}
	init.WriteString("  ret void\n}\n\n")
	return g.String(), init.String()
}

// loadGlobal reads a static's current value as a boxed ptr: a raw load
// plus box() at the C ABI boundary, a plain pointer load for an owned one.
func (e *emitter) loadGlobal(sb *strings.Builder, ent sem.EntityID) string {
	typ := e.r.Entity(ent).Type
	if owner, _ := e.r.StaticOwnerOf(ent); owner != sem.StaticOwned {
		ct := e.cType(typ)
		raw := e.newTmp()
		fmt.Fprintf(sb, "  %s = load %s, ptr @%s\n", raw, ct, e.r.StaticSymbolOf(ent))
		return e.box(sb, raw, typ, ct)
	}
	// The global still owns its own reference after handing one out, the
	// same contract as rt_field reading a record it does not give up.
	loaded := e.newTmp()
	fmt.Fprintf(sb, "  %s = load ptr, ptr %s\n", loaded, e.globalSym(ent))
	return e.irb(sb).Call(rtRetain, vptr(loaded)).Text
}

// storeGlobal is loadGlobal's write side for a `static mut`: the caller hands over an owned reference (Args[0] is consumed), so only the overwritten old value needs releasing, mirroring rt_setfield.
func (e *emitter) storeGlobal(sb *strings.Builder, ent sem.EntityID, v string) {
	typ := e.r.Entity(ent).Type
	if owner, _ := e.r.StaticOwnerOf(ent); owner != sem.StaticOwned {
		ct := e.cType(typ)
		raw := e.unbox(sb, v, typ, ct)
		fmt.Fprintf(sb, "  store %s %s, ptr @%s\n", ct, raw, e.r.StaticSymbolOf(ent))
		return
	}
	old := e.newTmp()
	fmt.Fprintf(sb, "  %s = load ptr, ptr %s\n", old, e.globalSym(ent))
	fmt.Fprintf(sb, "  store ptr %s, ptr %s\n", v, e.globalSym(ent))
	e.irb(sb).Call(rtRelease, vptr(old))
}

// sectionClause is `@section`'s LLVM text, empty when the declaration has
// none.
func sectionClause(name string) string {
	if name == "" {
		return ""
	}
	return fmt.Sprintf(", section %q", name)
}

// cLiteralText renders a static-initializable literal as the LLVM constant
// text of ct, for an export(C) static's global initializer.
func cLiteralText(lit sem.Literal, ct string) string {
	if ct == "float" || ct == "double" {
		f := 0.0
		switch {
		case lit.Float != nil:
			f, _ = lit.Float.Float64()
		case lit.Int != nil:
			f, _ = new(big.Float).SetInt(lit.Int).Float64()
		}
		return fmt.Sprintf("0x%X", math.Float64bits(f))
	}
	if lit.Int == nil {
		if lit.Bool {
			return "1"
		}
		return "0"
	}
	if lit.Int.IsInt64() {
		return lit.Int.String()
	}
	return fmt.Sprintf("%d", int64(lit.Int.Uint64()))
}
