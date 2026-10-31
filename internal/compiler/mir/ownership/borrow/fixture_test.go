package borrow

import (
	"kigumi/internal/compiler/hir"
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

type fixture struct {
	R         *sem.Result
	ByEnt     map[sem.EntityID]*midir.Func
	Funcs     []*midir.Func
	input     *ownquery.Input
	HasErrors func() bool
	Render    func() string
}

var fixtureBuilder func(*sem.Result) (*ownquery.Input, []*midir.Func, func() bool, func() string)

func SetFixtureBuilderForTest(build func(*sem.Result) (*ownquery.Input, []*midir.Func, func() bool, func() string)) {
	fixtureBuilder = build
}

func buildTypedFixture(r *sem.Result) *fixture {
	if fixtureBuilder == nil {
		panic("borrow fixture builder is not registered")
	}
	input, funcs, hasErrors, render := fixtureBuilder(r)
	p := &fixture{R: r, ByEnt: input.ByEnt, Funcs: funcs, input: input, HasErrors: hasErrors, Render: render}
	input.Funcs = func() []*midir.Func { return p.Funcs }
	return p
}

func typeFixture(r *sem.Result) *fixture {
	p := &fixture{R: r, ByEnt: map[sem.EntityID]*midir.Func{}}
	p.input = &ownquery.Input{
		R: r, ByEnt: p.ByEnt, Funcs: func() []*midir.Func { return p.Funcs },
		Unlowered: func(sem.EntityID) bool { return false },
		HasBody: func(id sem.EntityID) bool {
			_, instance := r.InstanceOf(id)
			return !instance && hir.Lowerable(r, id)
		},
	}
	return p
}

func (p *fixture) bodyByEnt(id sem.EntityID) (*midir.Func, bool) { f, ok := p.ByEnt[id]; return f, ok }
