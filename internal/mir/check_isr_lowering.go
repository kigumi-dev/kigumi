package mir

import (
	"kigumi/internal/diag"
	"kigumi/internal/sem"
)

// CheckInterruptLowering walks what each export(interrupt) handler reaches in the lowered
// program and reports the first instruction or runtime helper that allocates, releases, boxes, panics or blocks.
func CheckInterruptLowering(p *Program) []AllocClaim {
	c, ok := sem.CodeByName("interrupt-handler-lowering")
	if !ok {
		return nil
	}
	var out []AllocClaim
	for _, f := range p.Funcs {
		if f.Ent == 0 || p.R.Entity(f.Ent).Kind != sem.EntFn {
			continue
		}
		if p.R.Fn(f.Ent).Export != "interrupt" {
			continue
		}
		// A release is only reported once no allocation explains it.
		if claim, bad := p.isrLoweringPath(f, c, false); bad {
			out = append(out, claim)
		} else if claim, bad := p.isrLoweringPath(f, c, true); bad {
			out = append(out, claim)
		}
	}
	return out
}

func (p *Program) isrLoweringPath(handler *Func, c sem.Code, releases bool) (AllocClaim, bool) {
	type frame struct {
		f     *Func
		inert []bool
		via   []diag.Diagnostic
	}
	seen := map[isrVisit]bool{isrVisitKey(handler, nil): true}
	work := []frame{{f: handler}}
	for len(work) > 0 {
		fr := work[0]
		work = work[1:]
		t := p.R.Tree(fr.f.File)
		for _, blk := range fr.f.Blocks {
			for _, in := range blk.Insts {
				at := diag.At(t.File, t.Span(in.Node))
				if what := p.isrForbidden(fr.f, in, fr.inert, releases); what != "" {
					return p.isrClaim(handler, c, what, append(fr.via, diag.Diagnostic{Loc: at, Msg: what})), true
				}
				for _, g := range p.isrCallees(fr.f, in) {
					inert := p.isrInertArgs(fr.f, in, g, fr.inert)
					if k := isrVisitKey(g, inert); !seen[k] {
						seen[k] = true
						work = append(work, frame{g, inert, append(append([]diag.Diagnostic{}, fr.via...), diag.Diagnostic{Loc: at, Msg: "calls `" + p.isrFuncName(g) + "`"})})
					}
				}
			}
		}
	}
	return AllocClaim{}, false
}

func (p *Program) isrClaim(handler *Func, c sem.Code, what string, hops []diag.Diagnostic) AllocClaim {
	e := p.R.Entity(handler.Ent)
	t := p.R.Tree(e.File)
	d := diag.Diagnostic{Severity: diag.Error, Loc: diag.At(t.File, t.Span(e.Node)), Msg: sem.FillTemplate(c.Template, e.Name, what), Code: c.Num, Help: []string{c.Help}}
	for _, h := range hops {
		d = d.WithNote(h.Loc, h.Msg)
	}
	return AllocClaim{File: e.File, Diag: d}
}

// isrCallees are the lowered bodies one instruction runs in the same frame chain: named calls, known function values, asm `sym` targets.
func (p *Program) isrCallees(f *Func, in Inst) []*Func {
	switch in.Op {
	case OpCall:
		if g := p.ByEnt[in.Ent]; g != nil && !p.IsStdPrimitive(in.Ent) {
			return []*Func{g}
		}
	case OpCallValue:
		if g := p.isrValueTarget(f, in.Args[0]); g != nil {
			return []*Func{g}
		}
	case OpAsm:
		var out []*Func
		if info := p.R.AsmOf(f.File, in.Node); info != nil {
			for _, op := range info.Operands {
				if g := p.ByEnt[op.Sym]; op.Kind == sem.AsmSym && g != nil {
					out = append(out, g)
				}
			}
		}
		return out
	}
	return nil
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

// isrImmortal reports a value whose release never frees: a literal, a unit, a payload-free variant,
// a captureless closure or a parameter every caller on this path fed one of those.
func (p *Program) isrImmortal(f *Func, l LocalID, inert []bool) bool {
	return p.isrImmortalAt(f, l, inert, 16)
}

func (p *Program) isrImmortalAt(f *Func, l LocalID, inert []bool, depth int) bool {
	if depth == 0 {
		return false
	}
	defs := isrDefs(f, l)
	if isrParamIndex(f, l) >= 0 {
		if !isrInertParam(f, l, inert) {
			return false
		}
	} else if len(defs) == 0 {
		return false
	}
	for _, def := range defs {
		switch def.Op {
		case OpAlias, OpCopy, OpMove, OpShare:
			if !p.isrImmortalAt(f, def.Args[0], inert, depth-1) {
				return false
			}
		case OpConst, OpUnit, OpFnItem:
		case OpVariant, OpClosure:
			if len(def.Args) != 0 {
				return false
			}
		case OpBoxDirect:
			if !def.Fixed {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func (p *Program) isrFuncName(g *Func) string {
	if g.Ent != 0 {
		return p.R.Entity(g.Ent).Name
	}
	return g.Name
}
