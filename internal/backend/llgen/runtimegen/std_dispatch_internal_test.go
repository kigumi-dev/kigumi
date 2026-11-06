package runtimegen

import (
	"strings"
	"testing"
)

// opsFromBlocks is the registry a body table describes: every op it answers.
func opsFromBlocks(blocks []stdDispatchBlock) []Op {
	var ops []Op
	seen := map[string]bool{}
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			ops = append(ops, Op{ID: len(ops), Name: name})
		}
	}
	for _, b := range blocks {
		switch b.Kind {
		case stdDispatchOp:
			for _, name := range b.Names {
				add(name)
			}
		case stdDispatchCap:
			for _, arm := range b.Arms {
				for _, name := range arm.Names {
					add(name)
				}
			}
		}
	}
	return ops
}

// TestStdDispatchTableChecksTheRegistry pins the generator's guard rail: a live op with no body, an
// op the registry does not have, or a second body for one op is an error, so no case can go missing.
func TestStdDispatchTableChecksTheRegistry(t *testing.T) {
	blocks := func(t *testing.T) []stdDispatchBlock {
		t.Helper()
		parsed, err := parseStdDispatchBodies(stdDispatchBodies)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}
	ops := func(t *testing.T) []Op {
		t.Helper()
		return opsFromBlocks(blocks(t))
	}
	if err := checkStdDispatchBlocks(ops(t), blocks(t)); err != nil {
		t.Fatalf("the checked-in body table is invalid: %v", err)
	}
	first := func(t *testing.T, bs []stdDispatchBlock) (int, stdDispatchBlock) {
		t.Helper()
		for i, b := range bs {
			if b.Kind == stdDispatchOp {
				return i, b
			}
		}
		t.Fatal("the body table has no op block")
		return 0, stdDispatchBlock{}
	}
	t.Run("missing", func(t *testing.T) {
		live := ops(t)
		bs := blocks(t)
		i, op := first(t, bs)
		bs = append(bs[:i:i], bs[i+1:]...)
		err := checkStdDispatchBlocks(live, bs)
		if err == nil || !strings.Contains(err.Error(), "no body for live op(s) "+op.Names[0]) {
			t.Errorf("dropping %s: got %v", op.Names[0], err)
		}
	})
	t.Run("unknown", func(t *testing.T) {
		live := ops(t)
		bs := blocks(t)
		i, _ := first(t, bs)
		bs[i].Names = []string{"NOT_A_REGISTRY_OP"}
		err := checkStdDispatchBlocks(live, bs)
		if err == nil || !strings.Contains(err.Error(), "NOT_A_REGISTRY_OP is not a live registry op") {
			t.Errorf("renaming a body: got %v", err)
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		live := ops(t)
		bs := blocks(t)
		_, op := first(t, bs)
		bs = append(bs, op)
		err := checkStdDispatchBlocks(live, bs)
		if err == nil || !strings.Contains(err.Error(), "op "+op.Names[0]+" has two bodies") {
			t.Errorf("duplicating %s: got %v", op.Names[0], err)
		}
	})
}
