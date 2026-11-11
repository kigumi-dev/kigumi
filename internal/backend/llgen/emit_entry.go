package llgen

import (
	"fmt"
	"strings"
)

// The C entry calls main directly when its result is a direct Result, and boxes only an Err.
func (e *emitter) mainSym() string {
	switch {
	case e.p.Entry == nil:
		return ""
	case e.p.Entry.Entry == nil || e.directMain():
		return e.sym(e.p.Entry)
	}
	return e.useEntry(e.p.Entry)
}

func (e *emitter) directMain() bool {
	m := e.p.Entry
	if m.Entry == nil || len(m.Params) != 0 {
		return false
	}
	if _, scalar := e.ssigs[m].ret(); scalar {
		return false
	}
	s := e.directSig(m)
	if s == nil || s.Ret == nil || !s.Ret.isAdt() || e.mainOkTag(s.Ret) < 0 {
		return false
	}
	e.aggHelpers(s.Ret)
	return true
}

func (e *emitter) mainOkTag(a *aggInfo) int {
	for i, v := range a.Variants {
		if e.r.Entity(v.Ent).Name == "Ok" {
			return i
		}
	}
	return -1
}

// callDirectMain leaves %r holding null for an Ok and the boxed Result for an Err.
func (e *emitter) callDirectMain(sb *strings.Builder, main string) {
	a := e.directSig(e.p.Entry).Ret
	slot := e.stackSlot(sb, a.Name)
	if e.byValueAgg(a) {
		v := e.newTmp()
		fmt.Fprintf(sb, "  %s = call %s %s()\n  store %s %s, ptr %s\n", v, a.Name, main, a.Name, v, slot)
	} else {
		fmt.Fprintf(sb, "  call void %s(ptr %s)\n", main, slot)
	}
	tag, isOk := e.newTmp(), e.newTmp()
	fmt.Fprintf(sb, "  %s = load %s, ptr %s\n", tag, adtTag, e.tagPtr(sb, a, slot))
	fmt.Fprintf(sb, "  %s = icmp eq %s %s, %d\n", isOk, adtTag, tag, e.mainOkTag(a))
	fmt.Fprintf(sb, "  br i1 %s, label %%exit.ok, label %%exit.err\nexit.ok:\n  br label %%exit.done\nexit.err:\n", isOk)
	box := e.newTmp()
	fmt.Fprintf(sb, "  %s = call ptr %s(ptr %s)\n  br label %%exit.done\nexit.done:\n", box, e.aggBoxSym(a), slot)
	fmt.Fprintf(sb, "  %%r = phi ptr [ null, %%exit.ok ], [ %s, %%exit.err ]\n", box)
}

// An artifact with no entry still initializes here: an embedder calls its
// exported functions with no other chance to set the runtime up.
func (e *emitter) mainWrapper(main string) string {
	name := "main"
	switch {
	case e.freestanding:
		name = "kigumi_main"
	case e.entrySymbol != "":
		name = e.entrySymbol
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "define i32 @%s(i32 %%argc, ptr %%argv) {\n", name)
	if e.abiSym != "" {
		e.irb(&sb).Call(rtAbiCheck, vptr(e.abiSym))
	}
	sb.WriteString("  call void @kg_init_immortals()\n")
	e.irb(&sb).Call(rtInit, vi32text("%argc"), vptr("%argv"), vi32(e.typeCount()), vptr("@rt_types"))
	sb.WriteString("  call void @kg_init_statics()\n")
	if e.p.Entry == nil {
		sb.WriteString("  ret i32 0\n}\n")
		return sb.String()
	}
	if e.directMain() {
		e.callDirectMain(&sb, main)
	} else {
		fmt.Fprintf(&sb, "  %%r = call ptr %s()\n", main)
	}
	sb.WriteString("  call void @kg_fini_statics()\n")
	exitDesc, ctxDesc, wrapped, code := e.exitDescriptors()
	e.irb(&sb).CallAs("%c", rtExitCode, vptr("%r"), vptr(exitDesc), vptr(ctxDesc), vi64(wrapped), vi64(code))
	sb.WriteString("  ret i32 %c\n}\n")
	return sb.String()
}

// A type nothing in the program builds has no descriptor and no value, so null is exact for it.
func (e *emitter) exitDescriptors() (exit, ctx string, wrapped, code int) {
	exit, ctx = "null", "null"
	exitEnt := e.r.LangItem("ExitError")
	if _, built := e.descs[exitEnt]; exitEnt == 0 || !built {
		return
	}
	exit = e.desc(exitEnt)
	code = e.r.Field(e.r.FindField(exitEnt, "code")).Index
	if ctxEnt := e.r.LangItem("Context"); ctxEnt != 0 {
		if _, built := e.descs[ctxEnt]; built {
			ctx = e.desc(ctxEnt)
			wrapped = e.r.Field(e.r.FindField(ctxEnt, "wrapped")).Index
		}
	}
	return
}
