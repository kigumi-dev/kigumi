package borrow_test

import (
	"kigumi/internal/compiler/hir"
	mirbuild "kigumi/internal/compiler/mir/build"
	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/mir/ownership/borrow"
	"kigumi/internal/compiler/sem"
)

func init() {
	borrow.SetFixtureBuilderForTest(func(r *sem.Result) (*ownquery.Input, []*midir.Func, func() bool, func() string) {
		p := mirbuild.Program(r)
		input := &ownquery.Input{
			R: r, ByEnt: p.ByEnt, Funcs: func() []*midir.Func { return p.Funcs },
			Unlowered: p.Unlowered,
			HasBody: func(id sem.EntityID) bool {
				_, instance := r.InstanceOf(id)
				return !instance && hir.Lowerable(r, id)
			},
		}
		return input, p.Funcs, p.HasErrors, p.Render
	})
}
