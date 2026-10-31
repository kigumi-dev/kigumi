package vm

import (
	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
)

type funcSupport struct {
	why     string
	callees []*mir.Func
}

// supportIndex memoizes each function's own gap and callees so checking many roots (one per test) scans every body once.
type supportIndex struct {
	m     *Machine
	memo  map[*mir.Func]*funcSupport
	drops []*mir.Func
}

func newSupportIndex(prog *mir.Program) *supportIndex {
	m := &Machine{p: prog, r: prog.R, host: map[string]hostFn{}, sh: newShared()}
	m.registerAll()
	x := &supportIndex{m: m, memo: map[*mir.Func]*funcSupport{}}
	for ent, f := range prog.ByEnt {
		e := m.r.Entity(ent)
		if e.Kind == sem.EntFn && m.r.TypeDecl(e.Parent).Drop == ent {
			x.drops = append(x.drops, f)
		}
	}
	return x
}

func (x *supportIndex) of(f *mir.Func) *funcSupport {
	if s, ok := x.memo[f]; ok {
		return s
	}
	s := &funcSupport{}
	for _, blk := range f.Blocks {
		for _, in := range blk.Insts {
			if why := x.m.supportsInst(&in); why != "" {
				s.why = why
				x.memo[f] = s
				return s
			}
			s.callees = append(s.callees, x.m.callees(&in)...)
		}
	}
	x.memo[f] = s
	return s
}

func (x *supportIndex) check(roots ...*mir.Func) (bool, string) {
	seen := map[*mir.Func]bool{}
	var work []*mir.Func
	add := func(f *mir.Func) {
		if f != nil && !seen[f] {
			seen[f] = true
			work = append(work, f)
		}
	}
	for _, f := range roots {
		add(f)
	}
	for _, f := range x.drops {
		add(f)
	}
	for len(work) > 0 {
		f := work[0]
		work = work[1:]
		s := x.of(f)
		if s.why != "" {
			return false, s.why
		}
		for _, c := range s.callees {
			add(c)
		}
	}
	return true, ""
}
