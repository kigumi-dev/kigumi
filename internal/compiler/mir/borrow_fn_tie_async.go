package mir

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	"kigumi/internal/compiler/sem"
)

// tieCarrier reports whether a type parameter instantiated as t can keep a borrow: in its type, or in a Future, Task or capture.
func (c *borrowCheck) tieCarrier(t sem.TypeID) bool {
	return c.holdsBorrow(t) || c.holdsDeferred(t)
}

// asyncKeepsBorrow: an async fn's Future keeps every argument, so it borrows what a borrowed receiver or parameter does.
func (c *borrowCheck) asyncKeepsBorrow(info *sem.FnInfo) bool {
	return info.Recv == sem.RecvSelf || info.Recv == sem.RecvMut || slices.ContainsFunc(info.Params, func(p sem.EntityID) bool {
		return c.mayBorrowResult(c.p.R.Entity(p).Type)
	})
}

// tieWant is what a call's result or `&mut` target can take from a fn value's result: a type parameter it names, or
// any Future or Task when it holds one.
type tieWant struct {
	params []sem.EntityID
	async  bool
}

func (c *borrowCheck) wantOf(t sem.TypeID) tieWant {
	return tieWant{params: c.typeParamsOf(t), async: c.holdsAsync(t)}
}

func (w tieWant) empty() bool { return len(w.params) == 0 && !w.async }

func (w tieWant) fed(c *borrowCheck, ret sem.TypeID) bool {
	return sharesParam(c.typeParamsOf(ret), w.params) || w.async && c.holdsAsync(ret)
}

// carriesWanted: s's value holds a fn value, in a field, element or payload, whose result want takes in.
func (c *borrowCheck) carriesWanted(s callSlot, want tieWant) bool {
	t := c.f.Locals[s.arg].Type
	if sharesParam(s.params, want.params) && c.carriesBorrowMaker(t, map[sem.TypeID]bool{}) {
		return true
	}
	return want.async && c.carriesMaker(t, map[sem.TypeID]bool{}, c.holdsAsync)
}

// keepsDeferredArg: a result of type ret may keep a parameter's Future, Task or capture, and with it what that borrows.
func (c *borrowCheck) keepsDeferredArg(params []sem.EntityID, ret sem.TypeID) bool {
	return slices.ContainsFunc(params, func(p sem.EntityID) bool {
		t := c.p.R.Entity(p).Type
		return c.holdsDeferred(t) && c.mayCarry(ret, t)
	})
}

// targetKind is what lets a `&mut` target's place keep what a call stores: a lifetime, a type parameter, or only a
// Future, Task or capture, which keeps just a tied fn value's result (carryFlows passes plain values).
type targetKind uint8

const (
	targetNone targetKind = iota
	targetLifetime
	targetParam
	targetDeferred
)

func (c *borrowCheck) targetKindOf(l midir.LocalID) targetKind {
	t, place := c.f.Locals[l].Type, c.placeOf(l)
	switch {
	case c.declaredLifetime(t) != 0 || c.declaredLifetime(place) != 0:
		return targetLifetime
	case c.namesTypeParam(t) || c.namesTypeParam(place):
		return targetParam
	case c.holdsDeferred(t) || c.holdsDeferred(place):
		return targetDeferred
	}
	return targetNone
}

func (c *borrowCheck) placeKeeps(place sem.TypeID, v flowArg) bool {
	return c.trackedContainer(place) || v.tie != tieNone && c.holdsDeferred(place)
}
