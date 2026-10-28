package mir

import (
	"slices"

	"kigumi/internal/sem"
)

func (c *borrowCheck) checkRelayStores(in Inst) {
	for _, v := range c.relayStoreValues(in) {
		b := c.flowBorrow(v.arg, !v.contents)
		if b == nil {
			continue
		}
		if v.at != nil {
			part, ok := c.partInfo(v)
			if !ok {
				continue
			}
			b.owners = slices.DeleteFunc(b.owners, func(o ownerRef) bool {
				return !slices.ContainsFunc(part.owners, func(p ownerRef) bool { return o.owner == p.owner })
			})
		}
		if b = c.frameBorrows(b); b != nil {
			c.reportShared(in, v.arg, b)
			return
		}
	}
}

func (c *borrowCheck) relayStoreValues(in Inst) []flowArg {
	if in.Op != OpCall && in.Op != OpCallValue {
		return nil
	}
	defs := soleDefs(c.f)
	h, args := c.bodyCallee(in), in.Args
	if in.Op == OpCallValue {
		o := c.calleeOrigin(c.f, defs, in.Args[0])
		h, args = o.body, slices.Concat(o.captures, in.Args[1:])
		if h == nil {
			var out []flowArg
			for k, a := range in.Args[1:] {
				decl, _ := c.callParam(c.f, in, k+1)
				if v, ok := c.relayValue(a, decl, nil); ok {
					if c.flowBorrow(a, !v.contents) == nil || o.param > 0 && c.boundCalleeQuiet(c.f, o.param-1, k) {
						continue
					}
					out = append(out, v)
				}
			}
			return out
		}
	}
	if h == nil {
		return nil
	}
	var out []flowArg
	for k, a := range args {
		for _, s := range c.paramSites(paramKey{g: h, i: k, kind: siteOut}) {
			if s.callee == 0 || s.callee > len(args) || !c.frameCallee(c.f, defs, args[s.callee-1], 8) {
				continue
			}
			decl := h.Locals[h.Params[k]].Type
			if v, ok := c.relayValue(a, decl, s.path); ok {
				out = append(out, v)
			}
		}
	}
	return out
}

func (c *borrowCheck) relayValue(a LocalID, decl sem.TypeID, path stepPath) (flowArg, bool) {
	tt := c.p.R.Types
	if decl != 0 && !c.paramKeepsBorrow(decl) {
		return flowArg{}, false
	}
	v := flowArg{arg: a, contents: decl != 0 && tt.Kind(c.f.Locals[a].Type) == sem.KRef && c.declaredLifetime(decl) == 0}
	if len(path) > 0 {
		v.at = []stepPath{path}
	}
	return v, true
}
