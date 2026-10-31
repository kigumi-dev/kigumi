package mir

import (
	midir "kigumi/internal/compiler/mir/ir"
	opdef "kigumi/internal/compiler/mir/ops"
	ownquery "kigumi/internal/compiler/mir/ownership"
)

// isrCallbackArg is the index of the closure argument a std primitive call runs natively, if the op takes one.
func (p *Program) isrCallbackArg(in midir.Inst) (int, bool) {
	if !ownquery.IsStdPrimitive(p.R, in.Ent) {
		return 0, false
	}
	id, ok := in.StdOp()
	if !ok {
		id, ok = opdef.StdOpFor(StdKey(p.R, in.Ent, true))
	}
	if !ok || int(id) >= len(opdef.Registry.Ops) {
		return 0, false
	}
	cb := opdef.Registry.Ops[id].Callback
	return cb, cb >= 0 && cb < len(in.Args)
}

// isrValueTarget follows aliases of l back to the closure or function item that made it, nil when it cannot.
func (p *Program) isrValueTarget(f *midir.Func, l midir.LocalID) *midir.Func {
	for range 16 {
		def, ok := isrDef(f, l)
		if !ok {
			return nil
		}
		switch def.Op {
		case midir.OpAlias, midir.OpCopy, midir.OpMove, midir.OpShare:
			l = def.Args[0]
		case midir.OpClosure, midir.OpFnItem:
			return p.ByEnt[def.Ent]
		default:
			return nil
		}
	}
	return nil
}

// isrDef is l's one defining instruction; a local with none or several has no single source.
func isrDef(f *midir.Func, l midir.LocalID) (midir.Inst, bool) {
	defs := isrDefs(f, l)
	if len(defs) != 1 {
		return midir.Inst{}, false
	}
	return defs[0], true
}

func isrDefs(f *midir.Func, l midir.LocalID) []midir.Inst {
	var out []midir.Inst
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op.Yields() && in.Dst == l {
				out = append(out, in)
			}
		}
	}
	return out
}
