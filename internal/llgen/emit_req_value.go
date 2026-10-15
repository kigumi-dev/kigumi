package llgen

import (
	"fmt"
	"strings"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

func (e *emitter) requirementValue(sb *strings.Builder, in mir.Inst, args []string) string {
	if in.Str == "applied" {
		return e.appliedValue(sb, in, args)
	}
	sym := e.requirementAdapter(in.Ent, in.Str == "witness")
	return e.closureCtor(sb, sym, false, args, e.argArray(sb, args))
}

// appliedValue is a method bound to its witnesses, called with the captured receiver in front.
func (e *emitter) appliedValue(sb *strings.Builder, in mir.Inst, args []string) string {
	info := e.r.Fn(in.Ent)
	n := len(e.r.Types.Node(info.Sig).Args)
	move := info.Recv == sem.RecvMove
	async := info.Declared&sem.EffAsync != 0
	sym := fmt.Sprintf("@\"applied$%d$%t$%t\"", n, move, async)
	if !e.cio[sym] {
		e.cio[sym] = true
		e.helpers.WriteString(e.applyBoundAdapter(sym, n, move, async))
	}
	return e.closureCtor(sb, sym, false, args, e.argArray(sb, args))
}

func (e *emitter) applyBoundAdapter(sym string, n int, move, async bool) string {
	hoisting := e.hoisting
	e.hoisting = false
	defer func() { e.hoisting = hoisting }()
	var sb strings.Builder
	fmt.Fprintf(&sb, "define ptr %s(ptr %%env, ptr %%args) {\n", sym)
	e.irb(&sb).CallAs("%w", rtAt, vptr("%env"), vi64(0))
	e.irb(&sb).CallAs("%r", rtAt, vptr("%env"), vi64(1))
	rest := make([]string, n)
	for i := range rest {
		rest[i] = fmt.Sprintf("%%a%d", i)
		e.irb(&sb).CallAs(rest[i], rtAt, vptr("%args"), vi64(i))
	}
	recv := "%r"
	if move {
		e.irb(&sb).CallAs("%rc", rtCopy, vptr("%r"))
		recv = "%rc"
	} else if async {
		e.irb(&sb).Call(rtRetain, vptr("%r"))
	}
	arr := e.argArray(&sb, append([]string{recv}, rest...))
	e.irb(&sb).CallAs("%res", rtCallv, vptr("%w"), vi32(n+1), vptr(arr))
	e.irb(&sb).Call(rtRelease, vptr("%w"))
	e.irb(&sb).Call(rtRelease, vptr("%r"))
	sb.WriteString("  ret ptr %res\n}\n\n")
	return sb.String()
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
		recv := "%r"
		if e.r.Fn(req).Recv == sem.RecvMove {
			e.irb(&sb).CallAs("%rc", rtCopy, vptr("%r"))
			recv = "%rc"
		}
		arr := e.argArray(&sb, append([]string{recv}, rest...))
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
