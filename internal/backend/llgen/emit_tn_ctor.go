package llgen

import (
	"fmt"
	"hash/fnv"

	"kigumi/internal/compiler/sem"
)

// ctorID tells the constructors of runtime types apart: the kind in the top byte over a hash of the constructor's
// spelling, so lazy and eager std builds name the same constructor alike.
func (e *emitter) ctorID(t sem.TypeID) int64 {
	n := e.r.Types.Node(t)
	if n.Kind == sem.KConst {
		_, ty := e.r.Types.ConstValueBig(t)
		return e.constCtor(ty)
	}
	return e.ctorOf(n.Kind, e.r.CtorKey(t))
}

// constCtor is the constructor id of a const argument of type ty.
func (e *emitter) constCtor(ty sem.TypeID) int64 { return e.ctorOf(sem.KConst, e.r.CtorKey(ty)) }

func (e *emitter) ctorOf(kind sem.TypeKind, key string) int64 {
	h := fnv.New64a()
	h.Write([]byte(key))
	id := int64(kind)<<56 | int64(h.Sum64()&(1<<56-1))
	if e.ctorKeys == nil {
		e.ctorKeys = map[int64]string{}
	}
	if was, ok := e.ctorKeys[id]; ok && was != key {
		panic(fmt.Sprintf("internal error: runtime type constructors %q and %q hash alike", was, key))
	}
	e.ctorKeys[id] = key
	return id
}
