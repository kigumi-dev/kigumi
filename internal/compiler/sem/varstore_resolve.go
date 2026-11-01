package sem

import "slices"

type varResolution struct {
	value  TypeID
	source TypeID
	users  []TypeID
	linked bool
}

func (v *varStore) resolution(t TypeID) *varResolution {
	if v.resolved == nil {
		v.resolved = map[TypeID]*varResolution{}
	}
	if entry := v.resolved[t]; entry != nil {
		return entry
	}
	entry := &varResolution{}
	v.resolved[t] = entry
	return entry
}

func (v *varStore) depend(parent, child TypeID) {
	if v.r.Types.ContainsVar(child) {
		entry := v.resolution(child)
		entry.users = append(entry.users, parent)
	}
}

func (v *varStore) setBinding(i uint32, t TypeID) {
	if v.binding[i] == t {
		return
	}
	variable := v.r.Types.Var(i)
	if entry := v.resolved[variable]; entry != nil {
		v.invalidate(variable)
		if dependency := v.resolved[entry.source]; dependency != nil {
			dependency.users = slices.DeleteFunc(dependency.users, func(user TypeID) bool { return user == variable })
		}
		entry.source = 0
	}
	v.binding[i] = t
}

// Trial bindings can roll back, so even fully resolved results must retain their dependencies.
func (v *varStore) invalidate(t TypeID) {
	queue := []TypeID{t}
	for len(queue) > 0 {
		last := len(queue) - 1
		entry := v.resolved[queue[last]]
		queue = queue[:last]
		if entry.value == 0 {
			continue
		}
		entry.value = 0
		queue = append(queue, entry.users...)
	}
}

// ResolveMisses counts the inference-variable resolutions no cache could answer.
func (r *Result) ResolveMisses() int { return r.resolveMisses }
