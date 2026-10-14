package driver_test

import (
	"fmt"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

type mutator struct {
	user  []int
	types []sem.TypeID
	ents  map[sem.EntityKind][]sem.EntityID
	strs  []string
	prims []mir.PrimitiveId
	provs []mir.ProviderOpId
}

func newMutator(p *mir.Program) *mutator {
	m := &mutator{ents: map[sem.EntityKind][]sem.EntityID{}}
	seenType := map[sem.TypeID]bool{}
	seenStr := map[string]bool{}
	seenPrim := map[mir.PrimitiveId]bool{}
	seenProv := map[mir.ProviderOpId]bool{}
	addType := func(t sem.TypeID) {
		if !seenType[t] {
			seenType[t] = true
			m.types = append(m.types, t)
		}
	}
	for i, f := range p.Funcs {
		if f.Ent != 0 && p.R.Packages[p.R.Entity(f.Ent).Pkg].Path == "main" {
			m.user = append(m.user, i)
		}
		for _, l := range f.Locals {
			addType(l.Type)
		}
		for _, b := range f.Blocks {
			for _, in := range b.Insts {
				addType(in.Type)
				if in.Str != "" && !seenStr[in.Str] {
					seenStr[in.Str] = true
					m.strs = append(m.strs, in.Str)
				}
				if in.Prim != 0 && !seenPrim[in.Prim] {
					seenPrim[in.Prim] = true
					m.prims = append(m.prims, in.Prim)
				}
				if in.Prov != 0 && !seenProv[in.Prov] {
					seenProv[in.Prov] = true
					m.provs = append(m.provs, in.Prov)
				}
			}
		}
	}
	for i := range p.R.Entities {
		e := sem.EntityID(i)
		k := p.R.Entity(e).Kind
		if k != sem.EntFn || p.ByEnt[e] != nil || p.IsStdPrimitive(e) {
			m.ents[k] = append(m.ents[k], e)
		}
	}
	return m
}

type site struct {
	m     *mutator
	p     *mir.Program
	r     *chooser
	f     *mir.Func
	fnIdx int
	bi    int
	ii    int
}

func (s *site) blk() *mir.Block { return &s.f.Blocks[s.bi] }
func (s *site) inst() *mir.Inst { return &s.f.Blocks[s.bi].Insts[s.ii] }
func (s *site) hasInst() bool   { return s.ii >= 0 }

func (s *site) block() mir.BlockID {
	return mir.BlockID(s.r.Intn(len(s.f.Blocks) + 1))
}

func (s *site) where() string {
	if s.hasInst() {
		return fmt.Sprintf("%s b%d/%d", s.f.Name, s.bi, s.ii)
	}
	return fmt.Sprintf("%s b%d term", s.f.Name, s.bi)
}

type edit struct {
	name  string
	inst  bool
	apply func(s *site) string
}

func (m *mutator) pickFunc(p *mir.Program, r *chooser) int {
	if len(m.user) > 0 && r.draw(10) < 7 {
		return m.user[r.draw(len(m.user))]
	}
	if len(p.Comptime) > 0 && r.draw(40) == 0 {
		return len(p.Funcs) + r.draw(len(p.Comptime))
	}
	return r.draw(len(p.Funcs))
}

func (m *mutator) pickSite(p *mir.Program, r *chooser) *site {
	idx := m.pickFunc(p, r)
	f := allFuncs(p)[idx]
	s := &site{m: m, p: p, r: r, f: f, fnIdx: idx, ii: -1}
	s.bi = r.draw(len(f.Blocks))
	if n := len(s.blk().Insts); n > 0 {
		s.ii = r.draw(n)
	}
	return s
}
