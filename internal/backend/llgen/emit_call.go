package llgen

import (
	"fmt"
	"strings"

	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
)

func (e *emitter) callEntity(sb *strings.Builder, f *midir.Func, in midir.Inst, args []string, dst sem.TypeID) string {
	ent := in.Ent
	if e.r.Fn(ent).Naked {
		var types []sem.TypeID
		for _, a := range in.Args {
			types = append(types, f.Locals[a].Type)
		}
		return e.foreignCall(sb, ent, args, types)
	}
	if fn, ok := e.p.ByEnt[ent]; ok {
		t := e.newTmp()
		fmt.Fprintf(sb, "  %s = call ptr %s(%s)\n", t, e.sym(fn), joinArgs(args))
		return t
	}
	if e.r.Fn(ent).Abi != "" {
		var types []sem.TypeID
		for _, a := range in.Args {
			types = append(types, f.Locals[a].Type)
		}
		return e.foreignCall(sb, ent, args, types)
	}
	if iface := e.r.Entity(ent).Parent; e.r.Entity(iface).Kind == sem.EntInterface {
		arr := e.argArray(sb, args[1:])
		mode := map[sem.RecvKind]int{sem.RecvMut: 1, sem.RecvMove: 2}[e.r.Fn(ent).Recv]
		return e.irb(sb).Call(rtCallSlot, vptr(args[0]), vi32(e.slotOf(iface, ent)), vi32(len(args)-1), vptr(arr), vi32(mode)).Text
	}
	key := e.stdKey(ent)
	if key == "std/dl.Symbol.call" {
		return e.dlCall(sb, f, in, args)
	}
	if key == "std/error.as" {
		return e.errorAsCall(sb, args[0], dst)
	}
	if strings.HasPrefix(key, "std/ffi.") || strings.HasPrefix(key, "std/mem.volatile") {
		var types []sem.TypeID
		for _, a := range in.Args {
			types = append(types, f.Locals[a].Type)
		}
		if res, ok := e.ffiRecordCall(sb, key, args, types, dst); ok {
			return res
		}
	}
	if len(in.Args) == 1 {
		if res, ok := e.convertCall(sb, ent, key, args, f.Locals[in.Args[0]].Type, dst); ok {
			return res
		}
	}
	arr := e.argArray(sb, args)
	if op, ok := in.StdOp(); ok {
		return e.stdIdCall(sb, "", op, len(args), arr)
	}
	return e.irb(sb).Call(rtStd, vptr(e.stdKeyStr(key)), vi32(len(args)), vptr(arr)).Text
}

func (e *emitter) stdKey(fn sem.EntityID) string { return opdef.StdKey(e.r, fn, false) }
