package borrow

import (
	"testing"

	"kigumi/internal/compiler/sem"
)

func TestMergeIntoContainerSources(t *testing.T) {
	a, b := ownerRef{owner: 1}, ownerRef{owner: 2}
	const la, lb = sem.EntityID(7), sem.EntityID(8)
	cases := []struct {
		name      string
		dst, src  borrowInfo
		grew      bool
		hasOwner  bool
		lifetime  sem.EntityID
		owners    int
		fromStatc bool
	}{
		{"empty takes the source", borrowInfo{empty: true}, borrowInfo{hasOwner: true, owners: []ownerRef{a}}, true, true, 0, 1, false},
		{"empty source adds nothing", borrowInfo{hasOwner: true, owners: []ownerRef{a}}, borrowInfo{empty: true}, false, true, 0, 1, false},
		{"owners union", borrowInfo{hasOwner: true, owners: []ownerRef{a}}, borrowInfo{hasOwner: true, owners: []ownerRef{a, b}}, true, true, 0, 2, false},
		{"a parameter's borrow outlives a local one", borrowInfo{hasOwner: true, owners: []ownerRef{a}}, borrowInfo{lifetime: la}, false, true, 0, 1, false},
		{"a local borrow turns a parameter's local", borrowInfo{lifetime: la}, borrowInfo{hasOwner: true, owners: []ownerRef{b}}, true, true, 0, 1, false},
		{"same lifetime", borrowInfo{lifetime: la}, borrowInfo{lifetime: la}, false, false, la, 0, false},
		{"different lifetimes name none", borrowInfo{lifetime: la}, borrowInfo{lifetime: lb}, true, false, 0, 0, false},
		{"a static under a lifetime keeps it", borrowInfo{lifetime: la}, borrowInfo{fromStatic: true}, false, false, la, 0, false},
		{"a lifetime replaces a static", borrowInfo{fromStatic: true}, borrowInfo{lifetime: la}, true, false, la, 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dst, src := tc.dst, tc.src
			grew := mergeInto(&dst, &src)
			if grew != tc.grew || dst.hasOwner != tc.hasOwner || dst.lifetime != tc.lifetime || len(dst.owners) != tc.owners || dst.fromStatic != tc.fromStatc {
				t.Fatalf("got grew=%v %+v", grew, dst)
			}
			if mergeInto(&dst, &src) {
				t.Fatal("a second merge of the same source grew again")
			}
		})
	}
}

func TestMergeIntoKeepsMultiLifetimeCause(t *testing.T) {
	const multi = sem.TypeID(42)
	for _, tc := range []struct {
		name     string
		dst, src borrowInfo
	}{
		{"empty takes it", borrowInfo{empty: true}, borrowInfo{multi: multi}},
		{"a parameter's borrow takes it", borrowInfo{lifetime: 7}, borrowInfo{multi: multi}},
		{"kept against a later source", borrowInfo{multi: multi}, borrowInfo{lifetime: 7}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dst, src := tc.dst, tc.src
			mergeInto(&dst, &src)
			if dst.multi != multi {
				t.Fatalf("got %+v", dst)
			}
			if mergeInto(&dst, &src) {
				t.Fatal("a second merge of the same source grew again")
			}
		})
	}
}
