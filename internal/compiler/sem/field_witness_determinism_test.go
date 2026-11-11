package sem_test

import (
	"testing"
	"time"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/lang/diag"
	"kigumi/internal/toolchain/contract/schema"
)

// TestFieldWitnessDeterministic reruns the field-witness fixture and expects
// byte-identical diagnostics each time, catching unstable map iteration.
func TestFieldWitnessDeterministic(t *testing.T) {
	for _, name := range []string{"async_root_bridge", "async_root_bridge_witness", "async_root_bridge_closure", "async_root_bridge_summary", "async_root_bridge_late", "async_root_bridge_order", "async_root_bridge_nested", "async_root_bridge_result", "async_root_bridge_copy", "async_root_bridge_param_nested", "async_root_bridge_fnbound", "async_root_bridge_flow", "async_root_bridge_value", "async_root_bridge_scope"} {
		t.Run(name, func(t *testing.T) { checkDeterministic(t, semFixture(t, name)) })
	}
}

func checkDeterministic(t *testing.T, path string) {
	first := ""
	for i := range 5 {
		fx := parseFixture(t, path)
		mod := buildModule(t, fx)
		res := sem.CheckWith(mod, schema.GenerateDerive)
		var got string
		for j := 1; j < len(res.Files); j++ {
			got += diag.RenderAll(res, res.Diagnostics(res.Files[j].Tree))
		}
		if i == 0 {
			first = got
			continue
		}
		if got != first {
			t.Fatalf("run %d differs from run 0\n--- run 0 ---\n%s--- run %d ---\n%s", i, first, i, got)
		}
	}
}

func TestMethodFieldWriteFixpointTerminates(t *testing.T) {
	fx := parseFixture(t, semFixture(t, "async_root_bridge"))
	mod := buildModule(t, fx)
	done := make(chan struct{})
	go func() {
		sem.CheckWith(mod, schema.GenerateDerive)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("closeMethodFieldWrites did not converge in time")
	}
}
