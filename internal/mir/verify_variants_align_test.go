package mir

import (
	"testing"

	"kigumi/internal/sem"
)

func TestJoinAliasesSettles(t *testing.T) {
	some := variantSet{list: []sem.EntityID{1}, all: 2}
	named := vplace{root: 1}
	a := newFacts(3)
	a.setKnown(named, some)
	for range 2 {
		b := newFacts(3)
		b.setAt(2, named)
		b.setKnown(named, some)
		if a.join(b) {
			t.Fatal("an alias only the other side has changed the state")
		}
	}
}

func TestJoinKeepsFactsLearnedThroughAnotherName(t *testing.T) {
	some := variantSet{list: []sem.EntityID{1}, all: 2}
	temp, slot := vplace{root: 1}, vplace{root: 2}
	a, b := newFacts(3), newFacts(3)
	a.setAt(2, temp)
	a.setKnown(temp, some)
	b.setKnown(slot, some)
	a.join(b)
	if got, ok := a.known[slot]; !ok || !got.equal(some) {
		t.Fatalf("known after the join: %v", a.known)
	}
	if _, ok := a.at[2]; ok {
		t.Fatal("the alias outlived the join")
	}
}
