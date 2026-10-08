package mir

import (
	"kigumi/internal/diag"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

const maxSpecRepeats = 64

// specsPerRoot bounds what one call from ordinary code brings in, since nested calls can multiply the count.
const specsPerRoot = 256

// Only gen's own meetings up the chain count: three smaller ones mean its arguments keep growing.
func (s *specializer) runaway(ctx *spec, gen sem.EntityID, args []sem.TypeID) bool {
	size, repeats, smaller := s.argsSize(args), 0, 0
	for sp := ctx; sp != nil; sp = sp.parent {
		if sp.gen != gen {
			continue
		}
		repeats++
		if s.argsSize(sp.args) < size {
			smaller++
		}
	}
	return smaller >= 3 || repeats >= maxSpecRepeats
}

func (s *specializer) argsSize(args []sem.TypeID) int {
	n := 0
	for _, a := range args {
		n += typeSize(s.p.R.Types, a)
	}
	return n
}

func typeSize(tt *sem.TypeTable, t sem.TypeID) int {
	if t == 0 {
		return 0
	}
	n := tt.Node(t)
	size := 1 + typeSize(tt, n.Elem)
	for _, a := range n.Args {
		size += typeSize(tt, a)
	}
	return size
}

func charge(sp, ctx *spec) {
	if ctx == nil {
		sp.root = sp
		return
	}
	sp.root = ctx.root
	sp.root.brought++
}

// The warning goes once to the call that brought the root in, where the caller can act on it.
func (s *specializer) overBudget(ctx *spec) bool {
	if ctx == nil || ctx.root.brought < specsPerRoot {
		return false
	}
	root := ctx.root
	if !root.capped {
		root.capped = true
		site := root.sites[0]
		s.warn(site.f, site.f.Blocks[site.blk].Insts[site.idx].Node, "spec-budget", s.p.R.Entity(root.gen).Name, s.argsText(root.args), specsPerRoot)
	}
	return true
}

func (s *specializer) warn(f *Func, node syntax.NodeID, code string, args ...any) {
	c, ok := sem.CodeByName(code)
	if !ok {
		return
	}
	msg := sem.FillTemplate(c.Template, args...)
	if c.Help != "" {
		msg += "; " + c.Help
	}
	t := s.p.R.Tree(f.File)
	s.p.Diags = append(s.p.Diags, FileDiag{File: f.File, Diag: diag.Diagnostic{Severity: diag.Warning, Loc: diag.At(t.File, t.Span(node)), Msg: msg, Code: c.Num}})
}
