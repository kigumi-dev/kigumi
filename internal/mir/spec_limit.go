package mir

import (
	"kigumi/internal/diag"
	"kigumi/internal/sem"
	"kigumi/internal/syntax"
)

const maxSpecRepeats = 64

// A run that capped a root while discarding some of its instantiations repeats with a larger limit.
const maxReclaims = 2

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
	if ctx == nil || ctx.root.brought < specsPerRoot+s.bonus[ctx.root.key] {
		return false
	}
	ctx.refused = true
	root := ctx.root
	if root.scalar {
		return true
	}
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

// reclaim reports whether a capped root held instantiations that went unused for another reason; each raises its limit.
func (s *specializer) reclaim(bonus map[string]int) bool {
	reached := map[*spec]bool{}
	for _, sp := range s.live() {
		reached[sp] = true
	}
	hit := s.capAffected()
	again := false
	for _, sp := range s.all {
		if root := sp.root; !reached[sp] && !hit[sp] && root != sp && root.capped {
			bonus[root.key]++
			again = true
		}
	}
	return again
}

// A refused call leaves its caller boxing, and the caller's discarding passes up to the callers above it.
func (s *specializer) capAffected() map[*spec]bool {
	hit := map[*spec]bool{}
	var work []*spec
	for _, sp := range s.all {
		if sp.refused {
			hit[sp] = true
			work = append(work, sp)
		}
	}
	for len(work) > 0 {
		sp := work[len(work)-1]
		work = work[:len(work)-1]
		for _, site := range sp.sites {
			if caller := s.byFunc[site.f]; caller != nil && !hit[caller] {
				hit[caller] = true
				work = append(work, caller)
			}
		}
	}
	return hit
}

// rollback undoes the run; entities the semantic result minted stay, unused.
func (s *specializer) rollback(mark int) {
	for i := len(s.edits) - 1; i >= 0; i-- {
		e := s.edits[i]
		e.f.Blocks[e.blk].Insts[e.idx].Ent = e.was
	}
	out := s.p.Funcs[:0]
	for _, f := range s.p.Funcs {
		if s.byFunc[f] == nil {
			out = append(out, f)
			continue
		}
		delete(s.p.ByEnt, f.Ent)
	}
	s.p.Funcs = out
	s.p.Diags = s.p.Diags[:mark]
}
