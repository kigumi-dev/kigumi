package ownership

import (
	"testing"

	midir "kigumi/internal/compiler/mir/ir"
)

func TestCowVecCloneIsIndependent(t *testing.T) {
	var a CowVec[uint8]
	for l := range midir.LocalID(10000) {
		a.Set(l, uint8(l%7))
	}
	b := a.Clone()
	b.Set(5, 200)
	b.Set(9000, 99)
	a.Set(6, 100)
	if a.Get(5) != 5 || a.Get(9000) != uint8(9000%7) || b.Get(6) != 6 {
		t.Fatalf("clone shares writes: a5=%d a9000=%d b6=%d", a.Get(5), a.Get(9000), b.Get(6))
	}
	if b.Get(5) != 200 || b.Get(9000) != 99 || a.Get(6) != 100 {
		t.Fatalf("writes lost: b5=%d b9000=%d a6=%d", b.Get(5), b.Get(9000), a.Get(6))
	}
}

func TestCowVecJoinHandlesMissingLeaves(t *testing.T) {
	maxOf := func(cur, x uint8) uint8 { return max(cur, x) }
	var a, b CowVec[uint8]
	a.Set(3, 4)
	b.Set(3, 9)
	b.Set(70000, 2)
	if !a.Join(&b, maxOf) || a.Get(3) != 9 || a.Get(70000) != 2 {
		t.Fatalf("join: a3=%d a70000=%d", a.Get(3), a.Get(70000))
	}
	if a.Join(&b, maxOf) {
		t.Fatal("second join reported a change")
	}
	var zero CowVec[uint8]
	c := a.Clone()
	if c.Join(&zero, func(cur, _ uint8) uint8 { return cur + 1 }); c.Get(3) != 10 {
		t.Fatalf("join against an empty vec skipped a leaf: c3=%d", c.Get(3))
	}
}

func TestCowVecJoinSharedPartsStayIndependent(t *testing.T) {
	first := func(cur, x uint8) uint8 {
		if cur != 0 {
			return cur
		}
		return x
	}
	var a, b CowVec[uint8]
	for l := range midir.LocalID(10000) {
		a.Set(l, 1+uint8(l%7))
	}
	b.Set(9999, 50)
	if !b.joinIdent(&a, first) || b.Get(5) != 6 || b.Get(9999) != 50 {
		t.Fatalf("join: b5=%d b9999=%d", b.Get(5), b.Get(9999))
	}
	if b.nodeAt(0) != a.nodeAt(0) {
		t.Fatal("join copied a node the other side could share")
	}
	a.Set(5, 100)
	b.Set(6, 200)
	if b.Get(5) != 6 || a.Get(6) != 7 {
		t.Fatalf("shared leaf written in place: b5=%d a6=%d", b.Get(5), a.Get(6))
	}
	c := b.Clone()
	var d CowVec[uint8]
	d.joinIdent(&c, first)
	c.Set(7, 90)
	d.Set(8, 91)
	if b.Get(7) != 1+7%7 || d.Get(7) != 1+7%7 || b.Get(8) != 1+8%7 || c.Get(8) != 1+8%7 {
		t.Fatalf("shared node written in place: b7=%d d7=%d b8=%d c8=%d", b.Get(7), d.Get(7), b.Get(8), c.Get(8))
	}
}

func TestCowVecEachVisitsSetEntriesInOrder(t *testing.T) {
	var v CowVec[uint8]
	for _, l := range []midir.LocalID{9000, 3, 64, 4096} {
		v.Set(l, 1)
	}
	var got []midir.LocalID
	v.Each(func(l midir.LocalID, _ uint8) { got = append(got, l) })
	want := []midir.LocalID{3, 64, 4096, 9000}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
}

func TestOwnVecJoinKeepsAbsentApart(t *testing.T) {
	var a, b ownVec
	b.set(7, ownValid)
	a.join(&b)
	if a.get(7) != ownValid {
		t.Fatalf("absent joined with valid gave %v", a.get(7))
	}
	var c, d ownVec
	c.set(7, ownUninit)
	d.set(7, ownValid)
	c.join(&d)
	if c.get(7) != ownMaybe {
		t.Fatalf("uninit joined with valid gave %v", c.get(7))
	}
}
