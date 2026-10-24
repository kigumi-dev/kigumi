package mir

import (
	"kigumi/internal/diag"
	"kigumi/internal/sem"
)

// heldSpecsPerRoot only stops a blow-up: past it a held call is an error, never a shared fallback.
const heldSpecsPerRoot = 4096

// heldRoot is a body that must not box: a declared `noalloc` fn, an interrupt handler, or an allocation profile's root.
type heldRoot struct {
	text   string
	count  int
	capped bool
}

// markRooted finds the bodies a held root reaches through the calls, values and closures that name them.
func (s *specializer) markRooted() {
	s.rooted = map[*Func]*heldRoot{}
	var work []*Func
	add := func(f *Func, root *heldRoot) {
		if f != nil && s.rooted[f] == nil {
			s.rooted[f] = root
			work = append(work, f)
		}
	}
	r := s.p.R
	roots, profile := r.AllocProfileRoots()
	for _, ent := range roots {
		add(s.p.ByEnt[ent], &heldRoot{text: "`" + r.Entity(ent).Name + "`, which " + profile + " checks,"})
	}
	for _, f := range s.p.Funcs {
		switch {
		case f.Ent == 0 || r.Entity(f.Ent).Kind != sem.EntFn:
		case r.Fn(f.Ent).Export == "interrupt":
			add(f, &heldRoot{text: "the interrupt handler `" + r.Entity(f.Ent).Name + "`"})
		case s.p.declaredNoalloc(f):
			add(f, &heldRoot{text: "the `noalloc` fn `" + r.Entity(f.Ent).Name + "`"})
		}
	}
	for len(work) > 0 {
		f := work[len(work)-1]
		work = work[:len(work)-1]
		for _, blk := range f.Blocks {
			for _, in := range blk.Insts {
				if namesBody(in.Op) {
					add(s.p.ByEnt[in.Ent], s.rooted[f])
				}
			}
		}
	}
}

func (s *specializer) heldIn(f *Func, ctx *spec) *heldRoot {
	if ctx != nil {
		return ctx.held
	}
	return s.rooted[f]
}

// hold keeps sp specialized; a body already scanned retries the calls a budget refused.
func (s *specializer) hold(sp *spec, root *heldRoot) {
	if root == nil || sp.held != nil {
		return
	}
	sp.held = root
	root.count++
	if s.scanned[sp.fn] {
		for _, f := range sp.funcs() {
			s.scan(f, sp)
		}
	}
}

func (s *specializer) overHeldLimit(f *Func, in Inst, root *heldRoot, gen sem.EntityID, args []sem.TypeID) bool {
	if root.count < heldSpecsPerRoot {
		return false
	}
	if !root.capped {
		root.capped = true
		s.report(diag.Error, f, in.Node, "held-spec-limit", root.text, heldSpecsPerRoot, s.p.R.Entity(gen).Name, s.argsText(args))
	}
	return true
}
