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
	kCell
	kOpaque
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
type obj struct {
	nk     int
	i      int64
	hi     int64
	f      float64
	s      []byte
	fields []*obj
	fn     *mir.Func
	stdKey string
	lent   int
	env    *obj
	req    *reqCall
	inner  *obj
	op     string
	data   any
	chg    int64
	rc     int32
	c      rune
	ent    sem.EntityID
	prov   mir.ProviderOpId
	dyn    sem.TypeID
	k      kind
	b      bool
	bind   bool
	async  bool
}

var (
	unitObj  = &obj{k: kUnit, rc: -1}
	trueObj  = &obj{k: kBool, rc: -1, b: true}
	falseObj = &obj{k: kBool, rc: -1}
)

func newObj(k kind) *obj {
	if liveTrack.Load() {
		liveMade.Add(1)
	}
	return &obj{k: k, rc: 1}
}

func mkInt(i int64, nk int) *obj {
	if nk == 0 {
		nk = 64 | 256
	}
	v := newObj(kInt)
	v.i, v.nk = i, nk
	return v
}

func mkFloat(f float64, nk int) *obj {
	v := newObj(kFloat)
	if nk&255 == 32 {
		f = float64(float32(f))
	}
	v.f, v.nk = f, nk
	return v
}

func mkBool(b bool) *obj {
	if b {
		return trueObj
	}
	return falseObj
}

func mkStr(s []byte) *obj   { v := newObj(kStr); v.s = s; return v }
func mkBytes(s []byte) *obj { v := newObj(kBytes); v.s = s; return v }
func mkChar(c rune) *obj    { v := newObj(kChar); v.c = c; return v }

func mkArray(items []*obj) *obj {
	v := newObj(kArray)
	v.fields = append([]*obj{}, items...)
	return v
}

func mkRecord(ent sem.EntityID, fields []*obj) *obj {
	v := newObj(kRecord)
	v.ent, v.fields = ent, append([]*obj{}, fields...)
	return v
}

func mkVariant(ent sem.EntityID, payload []*obj) *obj {
	v := newObj(kVariant)
	v.ent, v.fields = ent, append([]*obj{}, payload...)
	return v
}

func mkCell(x *obj) *obj { v := newObj(kCell); v.inner = x; return v }

// Native boxes such a variant as its one immortal, with no allocation.
func payloadFree(v *obj) bool {
	v = deref(v)
	return v != nil && v.k == kVariant && len(v.fields) == 0
}

func mkOpaque(op string, data any) *obj { v := newObj(kOpaque); v.op, v.data = op, data; return v }

// deref follows cells, as every runtime entry point does.
func deref(v *obj) *obj {
	for v != nil && v.k == kCell {
		v = v.inner
	}
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
