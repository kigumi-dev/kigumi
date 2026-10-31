package ownership

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

type Input struct {
	R         *sem.Result
	ByEnt     map[sem.EntityID]*midir.Func
	Funcs     func() []*midir.Func
	Unlowered func(sem.EntityID) bool
	HasBody   func(sem.EntityID) bool
}
