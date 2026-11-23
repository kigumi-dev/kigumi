package sem

import (
	"slices"

	"kigumi/internal/compiler/syntax"
)

// Effect summaries: a node that declares `pure`/`noalloc` or has
// no body is fixed by its modifiers; every other body starts pure and
// noalloc and loses what its edges deny, iterated to a fixpoint.

func (r *Result) checkEffects() {
	nodes := r.effectNodes()
	r.settleBoxedParams()
	r.dropSharedValueAllocs(nodes)
	r.dropDirectBuildAllocs(nodes)
	r.settleEnvCaptures(nodes)
	r.inferSummaries(nodes)
	for _, id := range nodes {
		r.checkContracts(id)
	}
	r.inferDefault(nodes)
	r.inferOpSummaries(nodes)
	if r.noDefault {
		r.checkDefaultAllocator()
	}
	if r.recoverableOnly {
		r.checkRecoverableOnly(nodes)
	}
}

func (r *Result) effectNodes() []EntityID {
	var out []EntityID
	for id := 1; id < len(r.Entities); id++ {
		e := &r.Entities[id]
		if e.Flags&EfPoison != 0 {
			continue
		}
		switch e.Kind {
		case EntFn:
			if r.Fn(EntityID(id)).Body != 0 && r.Entities[e.Parent].Kind != EntInterface {
				out = append(out, EntityID(id))
			}
		case EntContract, EntImplicitMain, EntTest, EntClosure:
			out = append(out, EntityID(id))
		}
	}
	return out
}

// dropSharedValueAllocs drops the construction edges nothing allocates for.
func (r *Result) dropSharedValueAllocs(nodes []EntityID) {
	for _, id := range nodes {
		edges := r.edgesPtr(id)
		*edges = slices.DeleteFunc(*edges, func(e EffectEdge) bool {
			return e.Kind == EdgeAlloc && e.Why == whySharedValue
		})
	}
}

func (r *Result) dropDirectBuildAllocs(nodes []EntityID) {
	for _, id := range nodes {
		edges := r.edgesPtr(id)
		*edges = slices.DeleteFunc(*edges, func(e EffectEdge) bool {
			switch {
			case e.Captured && !r.directClosureEnt(id):
				return true
			case e.Why == whyInlined:
				return true
			case e.Why == whyBoxedArg:
				return !r.boxedParams[e.Param]
			case e.Kind != EdgeAlloc:
				return false
			case directCrossWhy(e.Why):
				return !r.DirectAgg(e.Type)
			case boxWhy(e.Why):
				return !r.boxCharged(e.Why, e.Type)
			}
			return e.Why == whyBuild && r.DirectAgg(e.Type)
		})
	}
}

func (r *Result) edgesPtr(id EntityID) *[]EffectEdge {
	if r.Entities[id].Kind == EntClosure {
		return &r.closure(id).Edges
	}
	return &r.Fn(id).Edges
}

func (r *Result) edgesOf(id EntityID) []EffectEdge {
	if r.Entities[id].Kind == EntClosure {
		return r.closure(id).Edges
	}
	return r.Fn(id).Edges
}

func (r *Result) summaryPtr(id EntityID) *EffectSummary {
	if r.Entities[id].Kind == EntClosure {
		return &r.closure(id).Summary
	}
	return &r.Fn(id).Summary
}

func (r *Result) fixedSummary(id EntityID) (EffectSummary, bool) {
	e := &r.Entities[id]
	if e.Kind == EntClosure {
		return EffectSummary{}, false
	}
	if e.Kind != EntFn && e.Kind != EntContract && e.Kind != EntImplicitMain && e.Kind != EntTest {
		return EffectSummary{}, true
	}
	info := r.Fn(id)
	if r.derivedInferred(id) {
		return EffectSummary{}, false
	}
	if info.Body == 0 || info.Declared&(EffPure|EffNoalloc) != 0 {
		s := EffectSummary{Pure: info.Declared&EffPure != 0, Noalloc: info.Declared&EffNoalloc != 0 || r.protoNoalloc(id), Known: true}
		if s.Pure {
			s.MutSelf, s.MutParams = r.signatureMutations(id)
			s.MutOnly = s.MutSelf || s.MutParams != 0
		}
		return s, true
	}
	return EffectSummary{}, false
}

func (r *Result) summary(id EntityID) EffectSummary {
	if s, ok := r.fixedSummary(id); ok {
		return s
	}
	return *r.summaryPtr(id)
}

func (r *Result) inferSummaries(nodes []EntityID) {
	var inferred []EntityID
	for _, id := range nodes {
		if _, fixed := r.fixedSummary(id); !fixed {
			inferred = append(inferred, id)
			*r.summaryPtr(id) = EffectSummary{Pure: true, Noalloc: true, Known: true}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, id := range inferred {
			s := r.summaryPtr(id)
			pure, noalloc := true, true
			mutSelf, mutParams := r.signatureMutations(id)
			mutOther := false
			for _, e := range r.edgesOf(id) {
				r.deferredClaims = r.deferredClaims || e.Kind == EdgeAlloc && demotableWhy(e.Why)
				noalloc = noalloc && r.edgeAllows(e, EffNoalloc)
				switch e.Kind {
				case EdgeMutateCaller:
					if self, bit, ok := r.ownMutation(id, e.Target); ok {
						mutSelf = mutSelf || self
						if !self {
							mutParams |= bit
						}
					} else {
						mutOther = true
					}
				case EdgeCall, EdgeWitness:
					ts := r.summary(e.Target)
					if ts.MutOnly {
						ms, mp, other := r.callMutations(e, ts)
						mutSelf = mutSelf || ms
						mutParams |= mp
						mutOther = mutOther || other
						continue
					}
					if ts.Pure {
						continue
					}
					mutOther = true
					continue
				default:
					if !r.edgeAllows(e, EffPure) {
						mutOther = true
					}
				}
			}
			pure = pure && !mutSelf && mutParams == 0 && !mutOther
			pure = pure || r.derivedInferred(id)
			mutOnly := !pure && !mutOther && (mutSelf || mutParams != 0)
			if pure != s.Pure || noalloc != s.Noalloc || mutSelf != s.MutSelf || mutParams != s.MutParams || mutOnly != s.MutOnly {
				s.Pure, s.Noalloc = pure, noalloc
				s.MutSelf, s.MutParams, s.MutOnly = mutSelf, mutParams, mutOnly
				changed = true
			}
		}
	}
}

// Origins of a call argument or receiver in the caller's frame.
const (
	argRootLocal   int16 = -1
	argRootUnknown int16 = -2
	argRootSelf    int16 = -3
)

// ownMutation names where a write inside id lands: the receiver, an explicit
// parameter, or neither (a captured value, which is never pure).
func (r *Result) ownMutation(id, target EntityID) (self bool, bit uint64, ok bool) {
	if target == 0 || r.Entities[id].Kind != EntFn {
		return false, 0, false
	}
	info := r.Fn(id)
	if target == info.SelfParam {
		return true, 0, true
	}
	if i := slices.Index(info.Params, target); i >= 0 && i < 64 {
		return false, 1 << uint(i), true
	}
	return false, 0, false
}

// signatureMutations reads a std API's declared mutability: `mut self` writes
// the caller's receiver cell, a `&mut` parameter may write what the caller
// passed. Std bodies are trusted contracts, so their bodies are not rechecked.
func (r *Result) signatureMutations(id EntityID) (self bool, params uint64) {
	if r.Entities[id].Kind != EntFn || !r.Packages[r.Entities[id].Pkg].Std {
		return false, 0
	}
	info := r.Fn(id)
	if info.SelfParam != 0 && r.Entities[info.SelfParam].Flags&EfMut != 0 {
		self = true
	}
	for i, p := range info.Params {
		if i >= 64 {
			break
		}
		t := r.Entities[p].Type
		if r.Types.Kind(t) == KRef && r.Types.RefMut(t) {
			params |= 1 << uint(i)
		}
	}
	return self, params
}

// callMutations maps a mut-only callee's mutation positions onto the caller:
// the receiver and arguments rooted in caller storage become this body's
// mutation, a fresh local stays internal, and an unknown origin is unsafe.
func (r *Result) callMutations(e EffectEdge, ts EffectSummary) (self bool, params uint64, other bool) {
	add := func(root int16) {
		switch root {
		case argRootLocal:
		case argRootSelf:
			self = true
		case argRootUnknown:
			other = true
		default:
			if root >= 0 && root < 64 {
				params |= 1 << uint(root)
			} else {
				other = true
			}
		}
	}
	if ts.MutSelf {
		if !e.RecvKnown {
			other = true
		} else {
			add(e.RecvRoot)
		}
	}
	for i := 0; i < 64; i++ {
		if ts.MutParams&(1<<uint(i)) == 0 {
			continue
		}
		if i >= int(e.ArgCount) {
			other = true
			continue
		}
		add(e.ArgRoots[i])
	}
	return self, params, other
}

func (r *Result) edgeAllows(e EffectEdge, mod Effects) bool {
	switch e.Kind {
	case EdgeCall, EdgeWitness:
		s := r.summary(e.Target)
		if mod != EffPure {
			return s.Noalloc
		}
		if s.MutOnly {
			self, params, other := r.callMutations(e, s)
			return !other && !self && params == 0
		}
		return s.Pure
	case EdgeCallback:
		if e.Param != 0 && !slices.Contains(r.Fn(r.Entities[e.Param].Parent).CalledParams, e.Param) {
			return true
		}
		if e.Target != 0 {
			return allows(r.summary(e.Target), mod)
		}
		return e.Mods&mod != 0
	case EdgeParamCall, EdgeWitnessCall:
		return true
	case EdgeIO:
		return false
	case EdgeMutateCaller, EdgeUnsafe:
		return mod != EffPure
	case EdgeAlloc:
		return mod != EffNoalloc || r.undemoted(e)
	case EdgeDrop:
		return allows(r.dropEffect(e.Type), mod)
	}
	return true
}

func allows(s EffectSummary, mod Effects) bool {
	if mod == EffPure {
		return s.Pure
	}
	return s.Noalloc
}

func modName(mod Effects) string {
	if mod == EffPure {
		return "pure"
	}
	return "noalloc"
}

// argRoot traces an argument or receiver expression to the storage it names:
// a parameter of the enclosing body, a fresh local, or an unknown origin.
func (c *checker) argRoot(a syntax.NodeID) int16 {
	a = c.t.Unparen(a)
	for c.t.Kind(a) == syntax.BorrowExpr {
		a = c.t.View(a).AsBorrow().Operand().ID()
	}
	p, ok := c.buildPlace(a)
	if !ok {
		return argRootUnknown
	}
	ent := c.r.Entities[p.Root]
	switch ent.Kind {
	case EntParam:
		if ent.Parent != c.fn {
			return argRootUnknown
		}
		if p.Root == c.r.Fn(c.fn).SelfParam && ent.Flags&EfMut != 0 {
			return argRootSelf
		}
		if c.paramBackedByCaller(p.Root, a) {
			if i := slices.Index(c.r.Fn(c.fn).Params, p.Root); i >= 0 && i < 64 {
				return int16(i)
			}
			return argRootUnknown
		}
		return argRootLocal
	case EntLocal:
		// A reference local may point into caller storage; treating it as
		// fresh would let a pure body mutate the caller through it.
		if c.r.Types.Kind(ent.Type) == KRef {
			return argRootUnknown
		}
		return argRootLocal
	}
	return argRootUnknown
}

// paramBackedByCaller reports whether writing through p reaches storage the
// caller owns: a `&mut` parameter, or a by-value parameter whose field path
// opens a mutable reference. A plain `mut` by-value parameter is a local copy.
func (c *checker) paramBackedByCaller(p EntityID, n syntax.NodeID) bool {
	t := c.vars.resolve(c.r.Entities[p].Type)
	if c.r.Types.Kind(t) == KRef && c.r.Types.RefMut(t) {
		return true
	}
	if t := c.vars.resolve(c.info.Types[n]); c.r.Types.Kind(t) == KRef && c.r.Types.RefMut(t) {
		return true
	}
	for i := 0; n != 0 && i < 64; i++ {
		switch c.t.Kind(n) {
		case syntax.MemberExpr, syntax.BracketExpr:
			n = c.t.View(n).Base().ID()
			bt := c.vars.resolve(c.info.Types[n])
			if c.r.Types.Kind(bt) == KRef && c.r.Types.RefMut(bt) {
				return true
			}
		default:
			return false
		}
	}
	return false
}

// setCallOrigins records each argument's storage origin on the EdgeCall, so
// effect inference can tell a caller-visible mutation from a local one.
func (c *checker) setCallOrigins(from int, args []syntax.NodeID, recv syntax.NodeID) {
	edges := *c.edges
	if from < 0 || from >= len(edges) || edges[from].Kind != EdgeCall {
		return
	}
	e := &edges[from]
	if recv != 0 {
		e.RecvRoot, e.RecvKnown = c.argRoot(recv), true
	}
	if len(args) > len(e.ArgRoots) {
		e.ArgCount = 0
		return
	}
	e.ArgCount = uint8(len(args))
	for i, a := range args {
		e.ArgRoots[i] = c.argRoot(a)
	}
}
