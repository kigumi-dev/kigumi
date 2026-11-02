package sem

import "strings"

// mismatchLabels qualifies want/got with their declaring package when they share a bare name, so the message never
// reads "expected `Widget`, found `Widget`".
func (r *Result) mismatchLabels(want, got TypeID) (string, string) {
	we, ge := r.namedTypeEntity(want), r.namedTypeEntity(got)
	if we != 0 && ge != 0 && we != ge {
		wf, gf := &r.Entities[we], &r.Entities[ge]
		if wf.Name == gf.Name && wf.Pkg != gf.Pkg {
			return r.qualifiedTypeString(want, we), r.qualifiedTypeString(got, ge)
		}
	}
	// Lifetime parameters of different declarations can both be spelled `'a`; qualify each with its declaring item.
	if qualify := r.collidingLifetimeParams(want, got); qualify != nil {
		var wsb, gsb strings.Builder
		r.writeTypeQualifyingLifetimes(&wsb, want, qualify)
		r.writeTypeQualifyingLifetimes(&gsb, got, qualify)
		return wsb.String(), gsb.String()
	}
	return r.TypeString(want), r.TypeString(got)
}

func (r *Result) collidingLifetimeParams(a, b TypeID) map[EntityID]bool {
	names := map[string]EntityID{}
	for _, p := range r.Types.Params(a) {
		if r.typeParam(p).IsLifetime {
			names[r.Entities[p].Name] = p
		}
	}
	var out map[EntityID]bool
	for _, p := range r.Types.Params(b) {
		if !r.typeParam(p).IsLifetime {
			continue
		}
		if other, ok := names[r.Entities[p].Name]; ok && other != p {
			if out == nil {
				out = map[EntityID]bool{}
			}
			out[p], out[other] = true, true
		}
	}
	return out
}

func (r *Result) writeTypeQualifyingLifetimes(sb *strings.Builder, t TypeID, qualify map[EntityID]bool) {
	tt := r.Types
	n := tt.nodes[t]
	write := func(sb *strings.Builder, t TypeID) { r.writeTypeQualifyingLifetimes(sb, t, qualify) }
	switch {
	case n.Kind == KParam && qualify[n.Ent]:
		sb.WriteString(r.entityName(n.Ent))
		sb.WriteString(" (")
		sb.WriteString(r.entityName(r.Entities[n.Ent].Parent))
		sb.WriteString(")")
		return
	case n.Kind == KFn:
		r.writeFnWith(sb, n, write)
		return
	case n.Kind == KRef:
		sb.WriteString("&")
		if n.Lifetime != 0 && tt.nodes[n.Lifetime].Kind == KParam {
			write(sb, n.Lifetime)
			sb.WriteByte(' ')
		}
		if n.Flags&flagMut != 0 {
			sb.WriteString("mut ")
		}
		write(sb, n.Elem)
		return
	}
	if n.Kind == KNamed || n.Kind == KIface {
		if _, ok := tt.IsOption(t); !ok {
			if _, _, ok := tt.IsResult(t); !ok && len(n.Args) > 0 {
				sb.WriteString(r.entityName(n.Ent))
				sb.WriteString("[")
				for i, a := range n.Args {
					if i > 0 {
						sb.WriteString(", ")
					}
					r.writeTypeQualifyingLifetimes(sb, a, qualify)
				}
				sb.WriteString("]")
				return
			}
		}
	}
	r.writeType(sb, t)
}

func (r *Result) namedTypeEntity(t TypeID) EntityID {
	if t == 0 {
		return 0
	}
	tt := r.Types
	n := tt.nodes[t]
	if n.Kind != KNamed && n.Kind != KIface {
		return 0
	}
	if _, ok := tt.IsOption(t); ok {
		return 0
	}
	if _, _, ok := tt.IsResult(t); ok {
		return 0
	}
	return n.Ent
}

func (r *Result) qualifiedTypeString(t TypeID, ent EntityID) string {
	var sb strings.Builder
	if pkg := r.Entities[ent].Pkg; pkg != 0 && int(pkg) < len(r.Packages) {
		if path := r.Packages[pkg].Path; path != "" {
			sb.WriteString(path)
			sb.WriteString(".")
		}
	}
	sb.WriteString(r.entityName(ent))
	r.writeArgs(&sb, r.Types.nodes[t].Args)
	return sb.String()
}
