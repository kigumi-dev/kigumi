package sem

// WitnessID names the conformance of a type to an interface by their TypeKeys.
type WitnessID string

func (r *Result) WitnessID(impl, iface TypeID) WitnessID {
	return WitnessID(string(r.TypeKey(impl)) + " : " + string(r.TypeKey(iface)))
}

// WitnessArgKey spells how a call site produces a witness, by the DeclID of the function that implements it.
func (r *Result) WitnessArgKey(w WitnessArg) string {
	switch w.Kind {
	case WitnessLocal:
		return "$" + r.entityName(w.Ent)
	case WitnessBind:
		key := r.entityKey(w.Ent) + "<"
		for i, a := range w.Args {
			if i > 0 {
				key += ","
			}
			key += r.WitnessArgKey(a)
		}
		return key + ">"
	}
	return r.entityKey(w.Ent)
}
