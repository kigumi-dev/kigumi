package syntax

import (
	"os"
	"testing"

	"kigumi/internal/lang/token"
)

func benchTree(b *testing.B) (*Tree, []NodeID) {
	b.Helper()
	src, err := os.ReadFile("../../../std/prelude/prelude.kg")
	if err != nil {
		b.Fatal(err)
	}
	t := Parse(token.NewFile("prelude.kg", src))
	var lists []NodeID
	for v := range t.All() {
		if kindTable[v.Kind()].layout == layList {
			lists = append(lists, v.ID())
		}
	}
	return t, lists
}

var sink NodeID

//go:noinline
func childrenSlice(t *Tree, id NodeID) []NodeID {
	n := t.nodes[id]
	if kindTable[n.Kind].layout != layList {
		return nil
	}
	out := make([]NodeID, 0, n.Rhs-n.Lhs)
	for _, e := range t.extra[n.Lhs:n.Rhs] {
		out = append(out, NodeID(e))
	}
	return out
}

func BenchmarkChildrenSlice(b *testing.B) {
	t, lists := benchTree(b)
	b.ReportAllocs()
	var sum NodeID
	for b.Loop() {
		for _, l := range lists {
			for _, c := range childrenSlice(t, l) {
				sum += c
			}
		}
	}
	sink = sum
}

func BenchmarkListCursor(b *testing.B) {
	t, lists := benchTree(b)
	b.ReportAllocs()
	var sum NodeID
	for b.Loop() {
		for _, l := range lists {
			for c := t.List(l).Cursor(); c.Next(); {
				sum += c.ID()
			}
		}
	}
	sink = sum
}

func BenchmarkListIDsSeq(b *testing.B) {
	t, lists := benchTree(b)
	b.ReportAllocs()
	var sum NodeID
	for b.Loop() {
		for _, l := range lists {
			for c := range t.List(l).IDs() {
				sum += c
			}
		}
	}
	sink = sum
}

func BenchmarkListViewsSeq(b *testing.B) {
	t, lists := benchTree(b)
	b.ReportAllocs()
	var sum NodeID
	for b.Loop() {
		for _, l := range lists {
			for v := range t.List(l).Views() {
				sum += v.ID()
			}
		}
	}
	sink = sum
}
