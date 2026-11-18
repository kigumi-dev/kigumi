package sem

import "strings"

// IsDropFn reports whether id is the drop a resource type declares, which only its descriptor calls.
func (r *Result) IsDropFn(id EntityID) bool {
	id = r.GenericOf(id)
	return id != 0 && r.Entities[id].Kind == EntFn && r.isDropFn(id)
}

// ReceiverArgs binds the type parameters of method fn to recv, one instantiation of its owner, by matching
// the declared receiver type; a descriptor reaches the method with nothing else to bind them from.
func (r *Result) ReceiverArgs(fn EntityID, recv TypeID) (map[EntityID]TypeID, bool) {
	info := r.Fn(fn)
	if info.Recv == RecvNone || info.SelfParam == 0 {
		return nil, false
	}
	self := r.Entities[info.SelfParam].Type
	if r.Types.Kind(self) == KRef {
		self = r.Types.Node(self).Elem
	}
	want := map[EntityID]bool{}
	for _, p := range info.TypeParams {
		want[p] = !r.IsLifetimeParam(p)
	}
	bound := map[EntityID]TypeID{}
	if !r.Types.BindParams(self, recv, want, bound) {
		return nil, false
	}
	return bound, true
}

// InstanceConstArgs is the hidden const-parameter arguments, in ConstParams order, that a call of method fn
// through the descriptor of recv passes; false when recv does not fix every one of them.
func (r *Result) InstanceConstArgs(fn EntityID, recv TypeID) ([]ConstArg, bool) {
	params := r.constParamsOf(fn)
	if len(params) == 0 {
		return nil, true
	}
	bound, ok := r.ReceiverArgs(fn, recv)
	if !ok {
		return nil, false
	}
	out := make([]ConstArg, len(params))
	for i, p := range params {
		t, ok := bound[p]
		if !ok || !r.Types.IsConst(t) {
			return nil, false
		}
		v, ty := r.Types.ConstValueBig(t)
		out[i] = ConstArg{Kind: ConstArgLit, Value: v, Type: ty}
	}
	return out, true
}

// ConstParamsOf lists fn's const parameters in the order of its hidden const arguments.
func (r *Result) ConstParamsOf(fn EntityID) []EntityID { return r.constParamsOf(fn) }

// RuntimeKey spells t as TypeKey does, with each closure type also by its own entity: the identity of a type
// at runtime, where an instantiated closure is not the generic one.
func (r *Result) RuntimeKey(t TypeID) string {
	var sb strings.Builder
	r.writeKeyAs(&sb, t, nil, true)
	return sb.String()
}

// KeyTemplate spells t as RuntimeKey does, cut where a parameter hole selects occurs; holes names them in order.
func (r *Result) KeyTemplate(t TypeID, hole func(EntityID) bool) (frags []string, holes []EntityID) {
	var sb strings.Builder
	r.writeKeyAs(&sb, t, func(p EntityID) string {
		if hole(p) {
			holes = append(holes, p)
			return "\x00"
		}
		return r.entityName(p)
	}, true)
	return strings.Split(sb.String(), "\x00"), holes
}
