package sem

import "strings"

// holder is a closure that keeps a local alive, by capture or through a call's hidden provider argument.
type holder struct {
	closure   EntityID
	mode      CaptureMode
	immediate bool
}

func (r *Result) noteImplicitCapture(local, closure EntityID) {
	if r.ctxImplicit == nil {
		r.ctxImplicit = map[EntityID][]EntityID{}
	}
	r.ctxImplicit[local] = append(r.ctxImplicit[local], closure)
}

func (c *checker) holders(root EntityID) []holder {
	var out []holder
	for _, site := range c.captureSites[root] {
		out = append(out, holder{site.ctx.ent, site.ctx.list[site.idx].Mode, site.ctx.immediate})
	}
	for _, cl := range c.r.ctxImplicit[root] {
		info := c.r.closure(cl)
		for _, cap := range info.Captures {
			if cap.Local == root {
				out = append(out, holder{cl, cap.Mode, info.Immediate})
			}
		}
	}
	return out
}

func (r *Result) isCtxProvider(ent EntityID) bool {
	e := &r.Entities[ent]
	return e.Kind == EntLocal && strings.HasPrefix(e.Name, "$ctx")
}

// A provider is captured by closures only once the context fixpoint has run, so its sites settle after it.
func (c *checker) deferredToContext(roots []EntityID) bool {
	for _, root := range roots {
		if c.r.isCtxProvider(root) {
			return true
		}
	}
	return false
}

func (r *Result) settleCtxExclusive() {
	for _, f := range r.ctxDeferred {
		f()
	}
	r.ctxDeferred = nil
}

// A provider local has no source name, so a diagnostic names its type.
func (c *checker) displayName(ent EntityID) string {
	if c.r.isCtxProvider(ent) {
		return c.r.TypeString(c.r.Entities[ent].Type)
	}
	return c.r.Entities[ent].Name
}
