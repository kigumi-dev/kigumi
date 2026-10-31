package vm

import (
	"fmt"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// lowered is ent's body; reaching a body Build left out as unreachable is a compiler fault, never a primitive.
func (m *Machine) lowered(ent sem.EntityID) (*midir.Func, bool) {
	f, ok := m.p.ByEnt[ent]
	if !ok && m.p.Unlowered(ent) {
		panic(fmt.Sprintf("vm: %s has a body mir.Build did not lower", m.r.Entity(ent).Name))
	}
	return f, ok
}
