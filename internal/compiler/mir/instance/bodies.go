// Package instance holds what the runtime knows of one instantiation: its identity as an interned type, the
// environment a generic body receives, and the bodies plan specialized for a descriptor.
package instance

import (
	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// bodyKey names a witness or drop fn reached through the descriptor of one owner instantiation, by its identity.
type bodyKey struct {
	Fn   sem.EntityID
	Type sem.TypeID
}

// Bodies holds the bodies plan specialized for a descriptor, which a SIMD lane shape needs; a fn missing here
// runs its generic body with the descriptor as its environment.
type Bodies struct {
	canon Canon
	byKey map[bodyKey]*midir.Func
	fns   map[sem.EntityID]bool
}

func (b *Bodies) Put(r *sem.Result, fn sem.EntityID, inst sem.TypeID, body *midir.Func) {
	if b.byKey == nil {
		b.byKey, b.fns = map[bodyKey]*midir.Func{}, map[sem.EntityID]bool{}
	}
	b.byKey[bodyKey{fn, b.canon.Of(r, inst)}] = body
	b.fns[fn] = true
}

func (b *Bodies) Get(r *sem.Result, fn sem.EntityID, inst sem.TypeID) *midir.Func {
	if b == nil || !b.fns[fn] {
		return nil
	}
	return b.byKey[bodyKey{fn, b.canon.Of(r, inst)}]
}

// Specialized reports whether some instantiation runs its own body of fn, so the generic one cannot stand in.
func (b *Bodies) Specialized(fn sem.EntityID) bool { return b != nil && b.fns[fn] }
