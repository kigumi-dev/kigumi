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
	if in.Op != midir.OpCall && in.Op != midir.OpCallValue {
		return nil
	}
	defs := c.soleDefMap()
	h, args := c.bodyCallee(in), in.Args
	if in.Op == midir.OpCallValue {
		o := c.calleeOrigin(c.f, defs, in.Args[0])
		h, args = o.body, slices.Concat(o.captures, in.Args[1:])
		if h == nil {
			var out []relayStore
			sig := c.slotFnType(c.f.Locals[in.Args[0]].Type)
			for k, a := range in.Args[1:] {
				decl, _ := c.callParam(c.f, in, k+1)
				if v, ok := c.relayValue(a, decl, c.p.R.Types.FnParamKeepsRef(sig, k), nil); ok {
					if c.flowBorrow(a, !v.contents) == nil || o.isParam && c.boundCalleeQuiet(c.f, o.param, k) {
						continue
					}
					out = append(out, relayStore{v, c.callerLifetimeParam(sig, k)})
				}
			}
			return out
		}
	}
	if h == nil {
		return nil
	}
	var out []relayStore
	for k, a := range args {
		for _, s := range c.paramSites(paramKey{g: h, i: k, kind: siteOut}) {
			if !c.frameTarget(c.f, defs, s.callee, args, 8) {
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
