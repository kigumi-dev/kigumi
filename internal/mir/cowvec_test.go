package mir

import "testing"

func TestCowVecCloneIsIndependent(t *testing.T) {
	var a cowVec[uint8]
	for l := range LocalID(10000) {
		a.set(l, uint8(l%7))
	}
	b := a.clone()
	b.set(5, 200)
	b.set(9000, 99)
	a.set(6, 100)
	if a.get(5) != 5 || a.get(9000) != uint8(9000%7) || b.get(6) != 6 {
		t.Fatalf("clone shares writes: a5=%d a9000=%d b6=%d", a.get(5), a.get(9000), b.get(6))
	}
	if b.get(5) != 200 || b.get(9000) != 99 || a.get(6) != 100 {
		t.Fatalf("writes lost: b5=%d b9000=%d a6=%d", b.get(5), b.get(9000), a.get(6))
	}
}

func TestCowVecJoinHandlesMissingLeaves(t *testing.T) {
	maxOf := func(cur, x uint8) uint8 { return max(cur, x) }
	var a, b cowVec[uint8]
	a.set(3, 4)
	b.set(3, 9)
	b.set(70000, 2)
	if !a.join(&b, maxOf) || a.get(3) != 9 || a.get(70000) != 2 {
		t.Fatalf("join: a3=%d a70000=%d", a.get(3), a.get(70000))
	}
	if a.join(&b, maxOf) {
		t.Fatal("second join reported a change")
	}
	var zero cowVec[uint8]
	c := a.clone()
	if c.join(&zero, func(cur, _ uint8) uint8 { return cur + 1 }); c.get(3) != 10 {
		t.Fatalf("join against an empty vec skipped a leaf: c3=%d", c.get(3))
	}
}

func TestCowVecJoinSharedPartsStayIndependent(t *testing.T) {
	first := func(cur, x uint8) uint8 {
		if cur != 0 {
			return cur
		}
		return x
	}
	var a, b cowVec[uint8]
	for l := range LocalID(10000) {
		a.set(l, 1+uint8(l%7))
	}
	b.set(9999, 50)
	if !b.joinIdent(&a, first) || b.get(5) != 6 || b.get(9999) != 50 {
		t.Fatalf("join: b5=%d b9999=%d", b.get(5), b.get(9999))
	}
	if b.nodeAt(0) != a.nodeAt(0) {
		t.Fatal("join copied a node the other side could share")
	}
	a.set(5, 100)
	b.set(6, 200)
	if b.get(5) != 6 || a.get(6) != 7 {
		t.Fatalf("shared leaf written in place: b5=%d a6=%d", b.get(5), a.get(6))
	}
	c := b.clone()
	var d cowVec[uint8]
	d.joinIdent(&c, first)
	c.set(7, 90)
	d.set(8, 91)
	if b.get(7) != 1+7%7 || d.get(7) != 1+7%7 || b.get(8) != 1+8%7 || c.get(8) != 1+8%7 {
		t.Fatalf("shared node written in place: b7=%d d7=%d b8=%d c8=%d", b.get(7), d.get(7), b.get(8), c.get(8))
	}
}

func TestCowVecEachVisitsSetEntriesInOrder(t *testing.T) {
	var v cowVec[uint8]
	for _, l := range []LocalID{9000, 3, 64, 4096} {
		v.set(l, 1)
	}
	var got []LocalID
	v.each(func(l LocalID, _ uint8) { got = append(got, l) })
	want := []LocalID{3, 64, 4096, 9000}
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
