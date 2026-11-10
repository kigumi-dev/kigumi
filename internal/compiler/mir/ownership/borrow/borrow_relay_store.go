package borrow

import (
	"slices"

	midir "kigumi/internal/compiler/mir/ir"
	ownquery "kigumi/internal/compiler/mir/ownership"
	"kigumi/internal/compiler/sem"
)

func (c *borrowCheck) checkRelayStores(in midir.Inst) {
	for _, v := range c.relayStoreValues(in) {
		b := c.flowBorrow(v.arg, !v.contents)
		if b == nil {
			continue
		}
		if v.at != nil {
			part, ok := c.partInfo(v.flowArg)
			if !ok {
				continue
			}
			b.owners = slices.DeleteFunc(b.owners, func(o ownerRef) bool {
				return !slices.ContainsFunc(part.owners, func(p ownerRef) bool { return o.owner == p.owner })
			})
		}
		if b = c.frameBorrows(b); b == nil {
			continue
		}
		if v.param != 0 {
			c.reportCallerLifetime(in, v.arg, b, v.param)
		} else {
			c.reportShared(in, v.arg, b)
		}
		return
	}
}

func (c *borrowCheck) relayStoreValues(in midir.Inst) []relayStore {
	switch in.Op {
	case midir.OpCall:
		return c.bodyRelays(c.bodyCallee(in), in.Args)
	case midir.OpCallValue:
	default:
		return nil
	}
	origins, ok := c.calleeOrigins(c.f, c.soleDefMap(), in.Args[0])
	if ok && !slices.ContainsFunc(origins, func(o calleeOrigin) bool { return o.body == nil }) {
		var out []relayStore
		for _, o := range origins {
			out = append(out, c.bodyRelays(o.body, o.params(in.Args[1:]))...)
		}
		return out
	}
	param, held := -1, false
	if ok && origins[0].isParam {
		param, held = origins[0].param, origins[0].held
	}
	var out []relayStore
	sig := c.slotFnType(c.f.Locals[in.Args[0]].Type)
	for k, a := range in.Args[1:] {
		decl, _ := c.callParam(c.f, in, k+1)
		if v, ok := c.relayValue(a, decl, c.p.R.Types.FnParamKeepsRef(sig, k), nil); ok {
			if c.flowBorrow(a, !v.contents) == nil || param >= 0 && c.boundCalleeQuiet(c.f, param, k, held) {
				continue
			}
			out = append(out, relayStore{v, c.callerLifetimeParam(sig, k)})
		}
	}
	return out
}

func (c *borrowCheck) bodyRelays(h *midir.Func, args []midir.LocalID) []relayStore {
	if h == nil {
		return nil
	}
	defs := c.soleDefMap()
	var out []relayStore
	for k, a := range args {
		for _, s := range c.paramSites(paramKey{g: h, i: k, kind: siteOut}) {
			if !c.frameTarget(c.f, defs, s.callee, localsOf(args), 8) {
				continue
			}
			prm := h.Locals[h.Params[k]]
			if v, ok := c.relayValue(a, prm.Type, c.paramKeepsRef(h, k), s.path); ok {
				out = append(out, relayStore{flowArg: v})
			}
		}
	}
	return out
}

func (c *borrowCheck) relayValue(a midir.LocalID, decl sem.TypeID, keepsRef bool, path ownquery.Path) (flowArg, bool) {
	tt := c.p.R.Types
	if decl != 0 && !keepsRef && !c.paramKeepsBorrow(decl) {
		return flowArg{}, false
	}
	v := flowArg{arg: a, contents: decl != 0 && tt.Kind(c.f.Locals[a].Type) == sem.KRef && !keepsRef}
	if len(path) > 0 {
		v.at = []ownquery.Path{path}
	}
	return v, true
}
