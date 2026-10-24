package llgen

import (
	"fmt"
	"strings"
)

// The C entry calls main boxed, through its entry when main carries a direct result.
func (e *emitter) mainSym() string {
	switch {
	case e.p.Entry == nil:
		return ""
	case e.p.Entry.Entry != nil:
		return e.useEntry(e.p.Entry)
	}
	return e.sym(e.p.Entry)
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
	fmt.Fprintf(&sb, "  %%r = call ptr %s()\n", main)
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
