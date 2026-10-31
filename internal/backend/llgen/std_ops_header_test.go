package llgen

import (
	"os"
	"testing"

	"kigumi/internal/compiler/mir"
)

const stdOpHeaderPath = "runtime/rt_std_ops.h"

// TestStdOpHeaderFresh regenerates rt_std_ops.h with UPDATE=1 and otherwise fails when it is stale.
func TestStdOpHeaderFresh(t *testing.T) {
	want := StdOpHeader(mir.Registry)
	if os.Getenv("UPDATE") == "1" {
		if err := os.WriteFile(stdOpHeaderPath, []byte(want), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	got, err := os.ReadFile(stdOpHeaderPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Errorf("%s is stale: run UPDATE=1 go test ./internal/backend/llgen -run StdOpHeaderFresh", stdOpHeaderPath)
	}
}
