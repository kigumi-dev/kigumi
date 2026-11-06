package llgen

import (
	"os"
	"strings"
	"testing"

	opdef "kigumi/internal/compiler/mir/ops"
)

const stdDispatchPath = "runtime/rt_std_dispatch.inc"

// TestStdDispatchFresh regenerates rt_std_dispatch.inc with UPDATE=1 and otherwise fails when it is stale.
func TestStdDispatchFresh(t *testing.T) {
	want, err := StdDispatch(opdef.Registry)
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("UPDATE") == "1" {
		if err := os.WriteFile(stdDispatchPath, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(stdDispatchPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale: run UPDATE=1 go test ./internal/backend/llgen -run StdDispatchFresh", stdDispatchPath)
	}
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
	if err := checkStdDispatchBlocks(opdef.Registry, blocks(t)); err != nil {
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
		bs := blocks(t)
		i, op := first(t, bs)
		bs = append(bs[:i:i], bs[i+1:]...)
		err := checkStdDispatchBlocks(opdef.Registry, bs)
		if err == nil || !strings.Contains(err.Error(), "no body for live op(s) "+op.Names[0]) {
			t.Errorf("dropping %s: got %v", op.Names[0], err)
		}
	})
	t.Run("unknown", func(t *testing.T) {
		bs := blocks(t)
		i, _ := first(t, bs)
		bs[i].Names = []string{"NOT_A_REGISTRY_OP"}
		err := checkStdDispatchBlocks(opdef.Registry, bs)
		if err == nil || !strings.Contains(err.Error(), "NOT_A_REGISTRY_OP is not a live registry op") {
			t.Errorf("renaming a body: got %v", err)
		}
	})
	t.Run("duplicate", func(t *testing.T) {
		bs := blocks(t)
		_, op := first(t, bs)
		bs = append(bs, op)
		err := checkStdDispatchBlocks(opdef.Registry, bs)
		if err == nil || !strings.Contains(err.Error(), "op "+op.Names[0]+" has two bodies") {
			t.Errorf("duplicating %s: got %v", op.Names[0], err)
		}
	})
}
