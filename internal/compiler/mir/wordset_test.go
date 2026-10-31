package mir

import (
	"math/rand"
	"testing"
)

func TestWordSetOperationsAgreeWithMaps(t *testing.T) {
	rng := rand.New(rand.NewSource(7))
	pick := func() ([]int32, wordSet, map[int32]bool) {
		var ids []int32
		m := map[int32]bool{}
		for range rng.Intn(40) {
			id := int32(rng.Intn(3)*500 + rng.Intn(200))
			ids = append(ids, id)
			m[id] = true
		}
		return ids, wordSetOf(ids), m
	}
	collect := func(s wordSet) map[int32]bool {
		out := map[int32]bool{}
		s.each(func(id int32) { out[id] = true })
		return out
	}
	same := func(a, b map[int32]bool) bool {
		if len(a) != len(b) {
			return false
		}
		for k := range a {
			if !b[k] {
				return false
			}
		}
		return true
	}
	for range 400 {
		_, sa, ma := pick()
		_, sb, mb := pick()
		union, inter, minus := map[int32]bool{}, map[int32]bool{}, map[int32]bool{}
		for k := range ma {
			union[k] = true
			if mb[k] {
				inter[k] = true
			} else {
				minus[k] = true
			}
		}
		for k := range mb {
			union[k] = true
		}
		if !same(collect(sa.union(sb)), union) || !same(collect(sa.inter(sb)), inter) || !same(collect(sa.minus(sb)), minus) {
			t.Fatalf("operations disagree for %v and %v", ma, mb)
		}
		for id := int32(0); id < 1200; id += 7 {
			if sa.has(id) != ma[id] {
				t.Fatalf("has(%d) = %v", id, sa.has(id))
			}
		}
		if !sa.equal(wordSetOf(keys(ma))) {
			t.Fatalf("equal fails for %v", ma)
		}
	}
}

func keys(m map[int32]bool) []int32 {
	var out []int32
	for k := range m {
		out = append(out, k)
	}
	return out
}
