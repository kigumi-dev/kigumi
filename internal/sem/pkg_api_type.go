package sem

import (
	"fmt"
	"strings"
)

// apiType includes private fields and payloads: they decide size, layout, copy and drop, which importers observe.
func (r *Result) apiType(d *APIDecl, id EntityID) {
	e := &r.Entities[id]
	info := r.typeDecl(id)
	d.Generics = r.apiParams(info.Params)
	d.Layout = fmt.Sprintf("form=%d layout=%s align=%d", info.Form, info.Layout, info.Align)
	for _, f := range info.Fields {
		fe := &r.Entities[f]
		d.Fields = append(d.Fields, fmt.Sprintf("%s %s %s", fe.Name, r.apiVis(fe.Vis), r.TypeString(fe.Type)))
	}
	for _, v := range info.Variants {
		var payload []string
		for _, p := range r.variant(v).Payload {
			payload = append(payload, r.TypeString(p))
		}
		d.Variants = append(d.Variants, r.Entities[v].Name+"("+strings.Join(payload, ",")+")")
	}
	for _, impl := range info.Impls {
		d.Impls = append(d.Impls, r.TypeString(impl.Iface))
	}
	if info.Underlying != 0 {
		d.Sig = r.TypeString(info.Underlying)
	}
	s := r.dropEffect(e.Type)
	d.Drop = fmt.Sprintf("pure=%v noalloc=%v known=%v", s.Pure, s.Noalloc, s.Known)
	if info.Drop != 0 && len(info.Params) > 0 {
		d.Body = r.bodyFingerprint(info.Drop)
	}
}
