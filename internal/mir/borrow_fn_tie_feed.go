package mir

import (
	"slices"

	"kigumi/internal/sem"
)

type tieKind uint8

const (
	tieNone tieKind = iota
	tieRef
	tieRecv
	tieRecvMut
	tieValue
	tieFn
)

// tieArgs keeps only the arguments whose type can reach a parameter the fn value's result may borrow.
func (c *borrowCheck) tieArgs(info *sem.FnInfo, slots []callSlot, want tieWant) []flowArg {
	feeds, all, found := c.makerFeeds(slots, want)
	if !found {
		return nil
	}
	var out []flowArg
	for i, s := range slots {
		v := flowArg{arg: s.arg, tie: c.tieKindOf(info, i, s)}
		decl, recv := s.exact, false
		switch v.tie {
		case tieNone:
			continue
		case tieRecv, tieRecvMut:
			decl, recv = 0, true
			if info.SelfParam != 0 {
				decl = c.p.R.Entity(info.SelfParam).Type
			}
		}
		hit, mut := all, all
		for _, p := range feeds {
			if c.reaches(decl, recv, p) {
				hit = true
				mut = mut || c.holdsMutRef(p)
			}
		}
		if hit {
			v.shared = !mut
			out = append(out, v)
		}
	}
	return out
}

func (c *borrowCheck) tieKindOf(info *sem.FnInfo, i int, s callSlot) tieKind {
	switch {
	case i == 0 && s.exact == 0 && info.Recv != sem.RecvNone:
		if info.Recv == sem.RecvMove {
			return tieNone
		}
		if info.Recv == sem.RecvMut {
			return tieRecvMut
		}
		return tieRecv
	case c.p.R.Types.Kind(s.exact) == sem.KRef:
		return tieRef
	case c.fnResult(s.exact) != 0 || c.slotFnType(s.exact) != 0:
		return tieFn
	case c.holdsBorrow(s.exact) || c.namesTypeParam(s.exact):
		return tieValue
	}
	return tieNone
}

// tieInfo: a reborrow of the receiver's place or a named reference, shared when the fn value only reads.
func (c *borrowCheck) tieInfo(v flowArg, generic, caller bool) (*borrowInfo, bool) {
	var b *borrowInfo
	ok := false
	switch v.tie {
	case tieNone, tieFn:
		return c.flowInfo(v)
	case tieRecv, tieRecvMut:
		return c.receiverPlaceInfo(v.arg, v.tie == tieRecvMut && !v.shared)
	case tieRef:
		b, ok = c.holders[v.arg]
		recv := sem.RecvNone
		if v.shared {
			recv = sem.RecvSelf
		}
		if ok && !generic && !caller {
			if r, isRef := c.referentBorrow(v.arg, b, recv); isRef {
				return r, true
			}
		}
		if generic && (!ok || !b.hasOwner) {
			return c.receiverPlaceInfo(v.arg, false)
		}
	case tieValue:
		b, ok = c.holders[v.arg]
	}
	if ok && v.shared {
		b = sharedBorrows(b)
	}
	return b, ok
}

func sharedBorrows(b *borrowInfo) *borrowInfo {
	if !slices.ContainsFunc(b.owners, func(o ownerRef) bool { return o.mut }) {
		return b
	}
	out := *b
	out.owners = slices.Clone(b.owners)
	for i := range out.owners {
		out.owners[i].mut = false
	}
	return &out
}

// makerFeeds: `all` marks a value only known to carry such a fn value inside, whose parameters are not seen; a type
// parameter a maker is fed joins want, so the maker building that argument (`g(f(x))`) feeds the result too.
func (c *borrowCheck) makerFeeds(slots []callSlot, want tieWant) (feeds []sem.TypeID, all, found bool) {
	want.params = slices.Clone(want.params)
	used := make([]bool, len(slots))
	for grown := true; grown; {
		grown = false
		for k, s := range slots {
			ft := c.slotFnType(s.exact)
			switch {
			case used[k]:
			case ft != 0:
				n := c.p.R.Types.Node(ft)
				if !want.fed(c, n.Elem) || !c.makesOwnBorrow(s.arg) {
					continue
				}
				found, used[k] = true, true
				for _, i := range c.feedingParams(s.arg, len(n.Args)) {
					feeds = append(feeds, n.Args[i])
					for _, p := range c.typeParamsOf(n.Args[i]) {
						if !slices.Contains(want.params, p) {
							want.params, grown = append(want.params, p), true
						}
					}
				}
			case c.carriesWanted(s, want):
				found, all, used[k] = true, true, true
			}
		}
	}
	return feeds, all, found
}

// feedingParams narrows to the parameters naming a lifetime or type parameter the result names when the fn item is known.
func (c *borrowCheck) feedingParams(a LocalID, n int) []int {
	every := make([]int, n)
	for i := range every {
		every[i] = i
	}
	fn := c.fnEntityOf(a)
	if fn == 0 || c.p.R.Entity(fn).Kind != sem.EntFn || c.p.isAsyncBody(fn) {
		return every
	}
	info := c.p.R.Fn(fn)
	ret := c.p.R.Types.Node(info.Sig).Elem
	named := c.p.R.Types.Params(ret)
	if info.RetLifetime != 0 {
		named = append(named, info.RetLifetime)
	} else if c.refIn(ret) {
		return every
	}
	if info.Recv != sem.RecvNone || len(info.Params) != n {
		return every
	}
	var out []int
	for i, p := range info.Params {
		ps := c.p.R.Types.Params(c.p.R.Entity(p).Type)
		if lt := c.p.R.Local(p).Lifetime; lt != 0 {
			ps = append(ps, lt)
		}
		if sharesParam(ps, named) {
			out = append(out, i)
		}
	}
	if len(out) == 0 {
		return every
	}
	return out
}
