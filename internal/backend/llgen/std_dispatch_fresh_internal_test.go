package llgen

import (
	"os"
	"testing"

	opdef "kigumi/internal/compiler/mir/ops"
)

const stdDispatchPath = "runtimegen/runtime/rt_std_dispatch.inc"

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
