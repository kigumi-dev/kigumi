package driver_test

import (
	"fmt"
	"math/rand"
	"strings"

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
	m  *mutator
	p  *mir.Program
	r  *rand.Rand
	f  *mir.Func
	bi int
	ii int
}

func (s *site) blk() *mir.Block { return &s.f.Blocks[s.bi] }
func (s *site) inst() *mir.Inst { return &s.f.Blocks[s.bi].Insts[s.ii] }
func (s *site) hasInst() bool   { return s.ii >= 0 }

func (s *site) local() mir.LocalID {
	if len(s.f.Locals) == 0 || s.r.Intn(24) == 0 {
		return mir.LocalID(len(s.f.Locals) + s.r.Intn(2))
	}
	return mir.LocalID(s.r.Intn(len(s.f.Locals)))
}

func (s *site) typ() sem.TypeID {
	if s.r.Intn(12) == 0 {
		return sem.TypeID(s.r.Intn(s.p.R.Types.Len()))
	}
	return s.m.types[s.r.Intn(len(s.m.types))]
}

func (s *site) ent(k sem.EntityKind) (sem.EntityID, bool) {
	pool := s.m.ents[k]
	if len(pool) == 0 {
		return 0, false
	}
	return pool[s.r.Intn(len(pool))], true
}

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

func (m *mutator) pickFunc(p *mir.Program, r *rand.Rand) *mir.Func {
	if len(m.user) > 0 && r.Intn(10) < 7 {
		return p.Funcs[m.user[r.Intn(len(m.user))]]
	}
	if len(p.Comptime) > 0 && r.Intn(40) == 0 {
		return p.Comptime[r.Intn(len(p.Comptime))].Func
	}
	return p.Funcs[r.Intn(len(p.Funcs))]
}

func (m *mutator) pickSite(p *mir.Program, r *rand.Rand) *site {
	f := m.pickFunc(p, r)
	s := &site{m: m, p: p, r: r, f: f, bi: r.Intn(len(f.Blocks)), ii: -1}
	if n := len(s.blk().Insts); n > 0 {
		s.ii = r.Intn(n)
	}
	return s
}

func (m *mutator) mutate(p *mir.Program, r *rand.Rand) string {
	n := 1
	if r.Intn(4) == 0 {
		n++
		if r.Intn(4) == 0 {
			n++
		}
	}
	var did []string
	for attempt := 0; len(did) < n && attempt < 80; attempt++ {
		s := m.pickSite(p, r)
		e := allEdits[r.Intn(len(allEdits))]
		if e.inst && !s.hasInst() {
			continue
		}
		if d := e.apply(s); d != "" {
			did = append(did, fmt.Sprintf("%s: %s", e.name, d))
		}
	}
	if len(did) == 0 {
		return "no edit applied"
	}
	return strings.Join(did, "; ")
}
