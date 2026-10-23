package sem

import "strings"

// overloadSig is "" for a lone name, else the parameter types, widened only while a sibling collides.
func (r *Result) overloadSig(ent EntityID) string {
	set := r.Fn(ent).Set
	if set == 0 || len(r.Overloads[set].Members) < 2 {
		return ""
	}
	members := r.Overloads[set].Members
	own := r.paramsKey(ent)
	if !r.siblingMatches(ent, members, r.paramsKey) {
		return own
	}
	full := r.fullKey(ent)
	if !r.siblingMatches(ent, members, r.fullKey) {
		return full
	}
	return full + r.boundsKey(ent)
}

func (r *Result) siblingMatches(ent EntityID, members []EntityID, key func(EntityID) string) bool {
	own := key(ent)
	for _, m := range members {
		if m != ent && key(m) == own {
			return true
		}
	}
	return false
}

func (r *Result) fullKey(ent EntityID) string {
	return r.paramsKey(ent) + "->" + r.signatureKey(ent, r.Fn(ent).Sig)
}

func (r *Result) boundsKey(ent EntityID) string {
	info := r.Fn(ent)
	var sb strings.Builder
	for _, group := range []struct {
		tag    string
		params []EntityID
	}{{"own", info.TypeParams}, {"recv", info.RecvParams}} {
		sb.WriteString("<" + group.tag)
		for _, p := range group.params {
			sb.WriteString(";")
			for _, c := range r.typeParam(p).Constraints {
				sb.WriteString(itoa(int(c.Kind)))
				if c.Type != 0 {
					sb.WriteString(":" + r.signatureKey(ent, c.Type))
				}
				sb.WriteString(",")
			}
		}
		sb.WriteString(">")
	}
	return sb.String()
}

func (r *Result) paramsKey(ent EntityID) string {
	info := r.Fn(ent)
	var sb strings.Builder
	switch info.Recv {
	case RecvSelf:
		sb.WriteString("self;")
	case RecvMut:
		sb.WriteString("mut self;")
	case RecvMove:
		sb.WriteString("move self;")
	}
	sb.WriteString("(")
	if info.Sig != 0 {
		r.writeKeys(&sb, r.Types.nodes[info.Sig].Args, r.numberingOf(ent))
	}
	sb.WriteString(")")
	return sb.String()
}
