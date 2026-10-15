package vm

import "kigumi/internal/sem"

type obj1 struct {
	obj
	a [1]*obj
}

type obj2 struct {
	obj
	a [2]*obj
}

type obj3 struct {
	obj
	a [3]*obj
}

type obj4 struct {
	obj
	a [4]*obj
}

// withFields allocates an aggregate and a copy of items together when they are few.
func withFields(k kind, ent sem.EntityID, items []*obj) *obj {
	var v *obj
	switch len(items) {
	case 0:
		v = &obj{fields: []*obj{}}
	case 1:
		o := &obj1{}
		v = &o.obj
		v.fields = o.a[:]
	case 2:
		o := &obj2{}
		v = &o.obj
		v.fields = o.a[:]
	case 3:
		o := &obj3{}
		v = &o.obj
		v.fields = o.a[:]
	case 4:
		o := &obj4{}
		v = &o.obj
		v.fields = o.a[:]
	default:
		v = &obj{fields: make([]*obj, len(items))}
	}
	copy(v.fields, items)
	v.k, v.rc, v.ent = k, 1, ent
	return born(v)
}
