// Package vm executes MIR directly with the C runtime's value
// model: reference-counted boxed values, the ownership plan of
// mir.Plan deciding copies and releases, and drop hooks running when the
// last reference goes.
package vm

import (
	"sync/atomic"

	"kigumi/internal/mir"
	"kigumi/internal/sem"
)

type kind uint8

const (
	kFree kind = iota
	kInt
	kFloat
	kBool
	kStr
	kBytes
	kChar
	kUnit
	kRecord
	kVariant
	kArray
	kClosure
	kBox
	kOpaque
	kCell
	kRawArray
)

// reqCall is a method value on an interface requirement; env[0] is the witness when witness is set.
type reqCall struct {
	name    string
	witness bool
	// move marks a `move self` requirement: each call hands the witness a copy.
	move bool
}

// obj is one runtime value; rc < 0 marks a static object that is never freed,
// and rc is the only field two lanes reach at once, so it moves atomically.
// Kinds that need more than this 64-byte core embed it in a larger struct
// (strObj, closureObj, opaqueObj) and are reached through the kind-checked accessors.
type obj struct {
	rc    int32
	ent   sem.EntityID
	k     kind
	b     bool
	bind  bool
	async bool
	live  bool
	numk  int16
	// i holds an Int, the bits of a Float, a Char, or a Box's dynamic type.
	i int64
	// w holds the high half of a wide Int, or the bytes last charged for any other value.
	w      int64
	fields []*obj
	inner  *obj
}

type strObj struct {
	obj
	s []byte
}

type closureObj struct {
	obj
	fn     *mir.Func
	stdKey string
	env    *obj
	req    *reqCall
	lent   int
	prov   mir.ProviderOpId
	dyn    sem.TypeID
}

type opaqueObj struct {
	obj
	op   string
	data any
}

var (
	unitObj  = &obj{k: kUnit, rc: -1}
	trueObj  = &obj{k: kBool, rc: -1, b: true}
	falseObj = &obj{k: kBool, rc: -1}
)

func newObj(k kind) *obj {
	switch k {
	case kStr, kBytes:
		o := &strObj{}
		o.k, o.rc = k, 1
		return born(&o.obj)
	case kClosure:
		o := &closureObj{}
		o.k, o.rc = k, 1
		return born(&o.obj)
	case kOpaque:
		o := &opaqueObj{}
		o.k, o.rc = k, 1
		return born(&o.obj)
	case kRawArray:
		o := &rawArrObj{}
		o.k, o.rc = k, 1
		return born(&o.obj)
	}
	return born(&obj{k: k, rc: 1})
}

func born(v *obj) *obj {
	if liveTrack.Load() {
		liveMade.Add(1)
		v.live = true
	}
	return v
}

func mkInt(i int64, nk int) *obj {
	if nk == 0 {
		nk = 64 | 256
	}
	if uint64(i-internMin) < internN {
		if v := internInt(i, nk); v != nil {
			return v
		}
	}
	v := newObj(kInt)
	v.i = i
	v.setNk(nk)
	return v
}

func mkFloat(f float64, nk int) *obj {
	v := newObj(kFloat)
	if nk&255 == 32 {
		f = float64(float32(f))
	}
	v.setFl(f)
	v.setNk(nk)
	return v
}

func mkBool(b bool) *obj {
	if b {
		return trueObj
	}
	return falseObj
}

func mkStr(s []byte) *obj   { v := newObj(kStr); v.setStr(s); return v }
func mkBytes(s []byte) *obj { v := newObj(kBytes); v.setStr(s); return v }
func mkChar(c rune) *obj    { v := newObj(kChar); v.setCh(c); return v }

func mkArray(items []*obj) *obj { return withFields(kArray, 0, items) }

func mkRecord(ent sem.EntityID, fields []*obj) *obj { return withFields(kRecord, ent, fields) }

func mkVariant(ent sem.EntityID, payload []*obj) *obj { return withFields(kVariant, ent, payload) }

func mkCell(x *obj) *obj { v := newObj(kCell); v.inner = x; return v }

// Native boxes such a variant as its one immortal, with no allocation.
func payloadFree(v *obj) bool {
	v = deref(v)
	return v != nil && v.k == kVariant && len(v.fields) == 0
}

func mkOpaque(op string, data any) *obj {
	v := newObj(kOpaque)
	o := v.opq()
	o.op, o.data = op, data
	return v
}

// The caller holds a reference, so a positive count cannot reach zero under
// it and an immortal one never moves: the load decides, the add follows.
func retain(v *obj) *obj {
	if v != nil && atomic.LoadInt32(&v.rc) > 0 {
		atomic.AddInt32(&v.rc, 1)
	}
	return v
}

func setImmortal(v *obj) { atomic.StoreInt32(&v.rc, -1) }
