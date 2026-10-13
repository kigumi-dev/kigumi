package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

func (e *emitter) requirementValue(sb *strings.Builder, in mir.Inst, args []string) string {
	sym := e.requirementAdapter(in.Ent, in.Str == "witness")
	return e.closureCtor(sb, sym, false, args, e.argArray(sb, args))
}

func (e *emitter) requirementAdapter(req sem.EntityID, witness bool) string {
	iface := e.r.Entity(req).Parent
	kind := "reqvalue"
	if witness {
		kind = "witnessvalue"
	}
	sym := fmt.Sprintf("@\"%s.%s$%s\"", e.qualifiedName(iface), e.r.Entity(req).Name, kind)
	if e.cio[sym] {
		return sym
	}
	e.cio[sym] = true
	hoisting := e.hoisting
	e.hoisting = false
	defer func() { e.hoisting = hoisting }()
	var sb strings.Builder
	fmt.Fprintf(&sb, "define ptr %s(ptr %%env, ptr %%args) {\n", sym)
	n := len(e.r.Types.Node(e.r.Fn(req).Sig).Args)
	recvSlot := 0
	if witness {
		recvSlot = 1
		e.irb(&sb).CallAs("%w", rtAt, vptr("%env"), vi64(0))
	}
	e.irb(&sb).CallAs("%r", rtAt, vptr("%env"), vi64(recvSlot))
	rest := make([]string, n)
	for i := range rest {
		rest[i] = fmt.Sprintf("%%a%d", i)
		e.irb(&sb).CallAs(rest[i], rtAt, vptr("%args"), vi64(i))
	}
	if witness {
		arr := e.argArray(&sb, append([]string{"%r"}, rest...))
		e.irb(&sb).CallAs("%res", rtCallv, vptr("%w"), vi32(n+1), vptr(arr))
		e.irb(&sb).Call(rtRelease, vptr("%w"))
	} else {
		arr := e.argArray(&sb, rest)
		e.irb(&sb).CallAs("%res", rtCallSlot, vptr("%r"), vi32(e.slotOf(iface, req)), vi32(n), vptr(arr), vi32(0))
	}
	e.irb(&sb).Call(rtRelease, vptr("%r"))
	sb.WriteString("  ret ptr %res\n}\n\n")
	e.helpers.WriteString(sb.String())
	return sym
}
