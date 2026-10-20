package mir

// isrCallbackArg is the index of the closure argument a std primitive call runs natively, if the op takes one.
func (p *Program) isrCallbackArg(in Inst) (int, bool) {
	if !p.IsStdPrimitive(in.Ent) {
		return 0, false
	}
	id, ok := in.StdOp()
	if !ok {
		id, ok = StdOpFor(StdKey(p.R, in.Ent, true))
	}
	if !ok || int(id) >= len(Registry.Ops) {
		return 0, false
	}
	cb := Registry.Ops[id].Callback
	return cb, cb >= 0 && cb < len(in.Args)
}

// isrValueTarget follows aliases of l back to the closure or function item that made it, nil when it cannot.
func (p *Program) isrValueTarget(f *Func, l LocalID) *Func {
	for range 16 {
		def, ok := isrDef(f, l)
		if !ok {
			return nil
		}
		switch def.Op {
		case OpAlias, OpCopy, OpMove, OpShare:
			l = def.Args[0]
		case OpClosure, OpFnItem:
			return p.ByEnt[def.Ent]
		default:
			return nil
		}
	}
	return nil
}

// isrDef is l's one defining instruction; a local with none or several has no single source.
func isrDef(f *Func, l LocalID) (Inst, bool) {
	defs := isrDefs(f, l)
	if len(defs) != 1 {
		return Inst{}, false
	}
	return defs[0], true
}

func isrDefs(f *Func, l LocalID) []Inst {
	var out []Inst
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if in.Op.Yields() && in.Dst == l {
				out = append(out, in)
			}
		}
	}
	return out
}
