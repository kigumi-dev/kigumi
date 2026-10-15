package lsp

import (
	"sync"

	"kigumi/internal/sem"
)

// conformIndex holds the interface/type conformances of a snapshot, so an
// implementation request is a map lookup instead of a conformance check per pair.
type conformIndex struct {
	res          *sem.Result
	ifaces       []sem.EntityID
	implementers map[sem.EntityID][]sem.EntityID
	witnesses    map[sem.EntityID][]sem.EntityID
	mu           sync.Mutex
	perType      map[sem.EntityID]*typeConform
}

// typeConform is what one type conforms to and which requirement each of its methods witnesses.
type typeConform struct {
	ifaces       []sem.EntityID
	requirements map[sem.EntityID][]sem.EntityID
}

func (snap *snapshot) conformance() *conformIndex {
	snap.confOnce.Do(func() { snap.conf = buildConformIndex(snap.res) })
	return snap.conf
}

func buildConformIndex(res *sem.Result) *conformIndex {
	c := &conformIndex{res: res, ifaces: declared(res, sem.EntInterface), implementers: map[sem.EntityID][]sem.EntityID{}, witnesses: map[sem.EntityID][]sem.EntityID{}, perType: map[sem.EntityID]*typeConform{}}
	for _, ty := range declared(res, sem.EntType) {
		for _, iface := range c.forType(ty).ifaces {
			c.implementers[iface] = append(c.implementers[iface], ty)
			ws, _ := conforms(res, ty, iface)
			for i, req := range res.Iface(iface).Reqs {
				c.witnesses[req] = append(c.witnesses[req], ws[i])
			}
		}
	}
	return c
}

// forType is memoized per type, since a method's owner can be a type the
// source does not declare (a predeclared type with extension methods).
func (c *conformIndex) forType(ty sem.EntityID) *typeConform {
	c.mu.Lock()
	defer c.mu.Unlock()
	if tc, ok := c.perType[ty]; ok {
		return tc
	}
	tc := &typeConform{requirements: map[sem.EntityID][]sem.EntityID{}}
	for _, iface := range c.ifaces {
		ws, ok := conforms(c.res, ty, iface)
		if !ok {
			continue
		}
		tc.ifaces = append(tc.ifaces, iface)
		for i, w := range ws {
			tc.requirements[w] = append(tc.requirements[w], c.res.Iface(iface).Reqs[i])
		}
	}
	c.perType[ty] = tc
	return tc
}

// implementations pairs interfaces with conforming types and requirements
// with the methods witnessing them, in either direction.
func (snap *snapshot) implementations(ent sem.EntityID) []sem.EntityID {
	res, c := snap.res, snap.conformance()
	e := res.Entity(ent)
	switch {
	case e.Kind == sem.EntInterface:
		return c.implementers[ent]
	case e.Kind == sem.EntType:
		return c.forType(ent).ifaces
	case isMethod(res, ent) && res.Entity(res.Fn(ent).Owner).Kind == sem.EntInterface:
		return c.witnesses[ent]
	case isMethod(res, ent):
		return c.forType(res.Fn(ent).Owner).requirements[ent]
	}
	return nil
}
