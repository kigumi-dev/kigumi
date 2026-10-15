package sem

import "kigumi/internal/syntax"

var elementIntrinsics = map[string]bool{"get": true, "first": true, "last": true, "takeAt": true, "push": true, "set": true}

func (c *checker) containerReceiver(fn EntityID, recvBase syntax.NodeID, writes bool) (place EntityID, elem TypeID, ok bool) {
	info := c.r.Fn(fn)
	if info.Recv == RecvNone || recvBase == 0 || !elementIntrinsics[c.r.Entities[fn].Name] {
		return 0, 0, false
	}
	t := c.vars.resolve(c.info.Types[recvBase])
	if n := c.r.Types.Node(t); n.Kind == KRef {
		t = c.vars.resolve(n.Elem)
	}
	_, elems, shaped := c.r.containerShape(t)
	if !shaped || !c.r.indexable(t) || !c.r.holdsFnPlace(elems[0]) {
		return 0, 0, false
	}
	place = c.r.fnChainEnt(c.f, recvBase)
	if place == 0 || writes && !c.r.concretePlace(place) {
		return 0, 0, false
	}
	return c.r.nestedFnOwner(fieldFnKey{Ent: place, Field: c.r.containerField(fieldElement)}), elems[0], true
}

func (c *checker) noteContainerIntrinsic(n syntax.NodeID, fn EntityID, args []syntax.NodeID, recvBase syntax.NodeID, ret TypeID) {
	name := c.r.Entities[fn].Name
	writes := name == "push" || name == "set"
	elemPlace, elem, ok := c.containerReceiver(fn, recvBase, writes)
	if !ok {
		return
	}
	if writes {
		if len(args) > 0 {
			c.flowComponent(elemPlace, elem, args[len(args)-1])
		}
		return
	}
	// A view, not a copy: a param's element place must reach its callers.
	res := c.r.callResultEnt(c.f, n, fn)
	c.r.fieldWitnessOf[fieldFnKey{Ent: res, Field: c.r.containerField(fieldPayload)}] = elemPlace
}

func (c *checker) trackLoopPlaces(pat, head syntax.NodeID) {
	t := c.vars.resolve(c.info.Types[head])
	if n := c.r.Types.Node(t); n.Kind == KRef {
		t = c.vars.resolve(n.Elem)
	}
	_, elems, shaped := c.r.containerShape(t)
	if !shaped || !c.r.indexable(t) || !c.r.holdsFnPlace(elems[0]) {
		return
	}
	if place := c.r.fnChainEnt(c.f, head); place != 0 {
		c.trackPatternPlaces(pat, c.r.nestedFnOwner(fieldFnKey{Ent: place, Field: c.r.containerField(fieldElement)}))
	}
}

func (r *Result) indexable(t TypeID) bool {
	n := r.Types.Node(t)
	return n.Kind == KNamed && (n.Ent == r.Types.arrayEnt && n.Ent != 0 || r.isFixedArray(t))
}
