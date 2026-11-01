package sem

import (
	"slices"
	"testing"

	"kigumi/internal/compiler/syntax"
)

func TestPipedFirstKeepsTheSlot(t *testing.T) {
	plan := newCallPlan([]syntax.NodeID{10, 20, 30}, []int{0, 1, 2})
	got := plan.pipedFirst(20)
	if want := []syntax.NodeID{20, 10, 30}; !slices.Equal(got.Eval, want) {
		t.Fatalf("Eval = %v, want %v", got.Eval, want)
	}
	if want := []int{1, 0, 2}; !slices.Equal(got.Place, want) {
		t.Fatalf("Place = %v, want %v", got.Place, want)
	}
	if want := []int{1, 0, 2}; !slices.Equal(got.Order, want) {
		t.Fatalf("Order = %v, want %v", got.Order, want)
	}
	if first := plan.pipedFirst(10); first != plan {
		t.Fatal("an operand already first must leave the plan alone")
	}
}
