package vm

import (
	"kigumi/internal/compiler/mir"
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

type funcSupport struct {
	why     string
	callees []*midir.Func
}

// supportIndex memoizes each function's own gap and callees so checking many roots (one per test) scans every body once.
type supportIndex struct {
	m     *Machine
	memo  map[*midir.Func]*funcSupport
	drops []*midir.Func
}

func newSupportIndex(prog *mir.Program) *supportIndex {
	m := &Machine{p: prog, r: prog.R, host: map[string]hostFn{}, sh: newShared()}
	m.registerAll()
	x := &supportIndex{m: m, memo: map[*midir.Func]*funcSupport{}}
	for ent, f := range prog.ByEnt {
		e := m.r.Entity(ent)
		if e.Kind == sem.EntFn && m.r.TypeDecl(e.Parent).Drop == ent {
			x.drops = append(x.drops, f)
		}
	}
	return x
}

func (x *supportIndex) of(f *midir.Func) *funcSupport {
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

func (x *supportIndex) check(roots ...*midir.Func) (bool, string) {
	seen := map[*midir.Func]bool{}
	var work []*midir.Func
	add := func(f *midir.Func) {
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
