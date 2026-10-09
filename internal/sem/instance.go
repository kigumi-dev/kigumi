package sem

// InstanceInfo's Args follow Gen's TypeParams, 0 for a lifetime.
type InstanceInfo struct {
	Gen  EntityID
	Args []TypeID
}

// The copy keeps gen's name and parameter entities, so std keys and declared types answer as gen's.
func (r *Result) Instantiate(gen EntityID, args []TypeID, key string) EntityID {
	if id, ok := r.instanceKeys[key]; ok {
		return id
	}
	if r.instances == nil {
		r.instances, r.instanceKeys = map[EntityID]InstanceInfo{}, map[string]EntityID{}
	}
	info := *r.Fn(gen)
	info.Sig = r.Types.Subst(info.Sig, r.InstanceSubst(gen, args))
	e := r.Entities[gen]
	e.Type, e.Detail = info.Sig, r.addFn(info)
	id := r.newEntity(e)
	r.instances[id] = InstanceInfo{Gen: gen, Args: append([]TypeID(nil), args...)}
	r.instanceKeys[key] = id
	return id
}

func (r *Result) InstanceOf(id EntityID) (InstanceInfo, bool) {
	in, ok := r.instances[id]
	return in, ok
}

func (r *Result) InstanceSubst(gen EntityID, args []TypeID) map[EntityID]TypeID {
	s := map[EntityID]TypeID{}
	for i, p := range r.Fn(gen).TypeParams {
		if i < len(args) && args[i] != 0 {
			s[p] = args[i]
		}
	}
	return s
}

// Specializable leaves out a body a descriptor or an interface table names by entity.
func (r *Result) Specializable(gen EntityID) bool {
	if gen == 0 || r.Entities[gen].Kind != EntFn || r.Entities[gen].Flags&EfPoison != 0 {
		return false
	}
	if r.Entities[r.Entities[gen].Parent].Kind == EntInterface {
		return false
	}
	info := r.Fn(gen)
	if info.Body == 0 || len(info.TypeParams) == 0 || !r.DirectCarried(gen) {
		return false
	}
	return info.Owner == 0 || r.Entities[info.Owner].Kind != EntType || r.typeDecl(info.Owner).Drop != gen
}

func (r *Result) SpecializeArgs(args []TypeID) bool {
	for _, a := range args {
		if a != 0 && r.DirectAgg(a) {
			return true
		}
	}
	return false
}

// SpecializesCalls reports whether a call hands a direct aggregate to a specializable callee, the only way a specialization limit can trip.
func (r *Result) SpecializesCalls() bool {
	for _, f := range r.Files {
		for _, call := range f.Calls {
			if call.Callee != 0 && len(call.Inst) > 0 && r.Specializable(call.Callee) && r.SpecializeArgs(call.Inst) {
				return true
			}
		}
	}
	return false
}
