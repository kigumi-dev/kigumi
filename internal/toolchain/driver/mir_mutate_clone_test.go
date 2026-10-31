package driver_test

import (
	"fmt"
	"slices"
	"strings"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
)

func cloneFunc(f *mir.Func) *mir.Func {
	nf := *f
	nf.Params = slices.Clone(f.Params)
	nf.Locals = slices.Clone(f.Locals)
	nf.Direct = slices.Clone(f.Direct)
	nf.Slot = slices.Clone(f.Slot)
	nf.PanicDrop = slices.Clone(f.PanicDrop)
	nf.Cleanup = make([][]mir.LocalID, len(f.Cleanup))
	for i, c := range f.Cleanup {
		nf.Cleanup[i] = slices.Clone(c)
	}
	if f.DSig != nil {
		s := *f.DSig
		s.Params = slices.Clone(s.Params)
		nf.DSig = &s
	}
	nf.Blocks = make([]mir.Block, len(f.Blocks))
	for i, b := range f.Blocks {
		nb := mir.Block{Insts: make([]mir.Inst, len(b.Insts)), Term: b.Term}
		for k, in := range b.Insts {
			in.Args = slices.Clone(in.Args)
			in.Strs = slices.Clone(in.Strs)
			nb.Insts[k] = in
		}
		nb.Term.Args = slices.Clone(b.Term.Args)
		nb.Term.Targets = slices.Clone(b.Term.Targets)
		nf.Blocks[i] = nb
	}
	return &nf
}

func cloneProgram(p *mir.Program) *mir.Program {
	c := &mir.Program{R: p.R, ByEnt: map[sem.EntityID]*mir.Func{}, Resources: p.Resources}
	mapped := map[*mir.Func]*mir.Func{}
	for _, f := range p.Funcs {
		nf := cloneFunc(f)
		mapped[f] = nf
		c.Funcs = append(c.Funcs, nf)
	}
	for ent, f := range p.ByEnt {
		if nf, ok := mapped[f]; ok {
			c.ByEnt[ent] = nf
		} else {
			c.ByEnt[ent] = f
		}
	}
	c.Entry = mapped[p.Entry]
	for _, cf := range p.Comptime {
		ncf := *cf
		ncf.Func = cloneFunc(cf.Func)
		if o, ok := mapped[cf.Owner]; ok {
			ncf.Owner = o
		}
		c.Comptime = append(c.Comptime, &ncf)
	}
	return c
}

func dumpByFunc(p *mir.Program) map[string]string {
	out := map[string]string{}
	guard("dump", func() {
		for _, chunk := range strings.SplitAfter(p.Dump(), "\n}\n") {
			if head, _, ok := strings.Cut(chunk, "{\n"); ok {
				out[head] = chunk
			}
		}
	})
	return out
}

func changedFuncs(base, mutant *mir.Program) []string {
	was := dumpByFunc(base)
	var out []string
	for head, text := range dumpByFunc(mutant) {
		old := was[head]
		if old == text {
			continue
		}
		lines, oldLines := strings.Split(text, "\n"), strings.Split(old, "\n")
		if len(lines) > 120 {
			first := 0
			for first < len(lines) && first < len(oldLines) && lines[first] == oldLines[first] {
				first++
			}
			lines = lines[max(0, first-30):min(len(lines), first+30)]
		}
		out = append(out, "mutant function:\n"+strings.Join(lines, "\n"))
	}
	return out
}

func changedLocals(base, mutant *mir.Program) []string {
	var out []string
	for k, f := range mutant.Funcs {
		if k >= len(base.Funcs) || len(base.Funcs[k].Locals) != len(f.Locals) {
			continue
		}
		for i, l := range f.Locals {
			if o := base.Funcs[k].Locals[i]; o.Type != l.Type || o.Cell != l.Cell || o.Borrowed != l.Borrowed || o.Cross != l.Cross {
				out = append(out, fmt.Sprintf("local %%%s of %s: %s -> %s (cell %v->%v, borrowed %v->%v)", l.Name, f.Name, mutant.R.TypeString(o.Type), mutant.R.TypeString(l.Type), o.Cell, l.Cell, o.Borrowed, l.Borrowed))
			}
		}
	}
	return out
}
