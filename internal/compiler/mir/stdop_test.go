package mir

import (
	"testing"

	opdef "kigumi/internal/compiler/mir/ops"
)

// The per-op tables are built from the registry at package init, in dependency order, so each one is populated before anything reads it.
func TestRegistryTablesArePopulated(t *testing.T) {
	run, _ := opdef.StdOpIdOf("BLOCKING_POOL_RUN")
	if refs := opdef.NativeTypes(run); len(refs) != 1 || refs[0].Name != "BlockingError" {
		t.Errorf("BLOCKING_POOL_RUN builds %v by name, want BlockingError", refs)
	}
	if !opdef.RuntimeNamedTypes()["Entry"] || !opdef.RuntimeNamedTypes()["StringBuilder"] {
		t.Error("the types the try primitives build by name are missing from RuntimeNamedTypes")
	}
	directs := 0
	for _, op := range opdef.Registry.Ops {
		if op.Direct != "" {
			directs++
		}
	}
	elems := 0
	for _, isElem := range elemOps {
		if isElem {
			elems++
		}
	}
	if directs != 37 || len(hoCallbackArg) != 12 || elems < 3 {
		t.Errorf("tables hold %d direct ops, %d callbacks, %d element ops", directs, len(hoCallbackArg), elems)
	}
}
