package ops_test

import (
	"kigumi/internal/compiler/mir/ops"
	"slices"
	"testing"
)

func TestRegistryNamesAreUnique(t *testing.T) {
	names := ops.BuiltinNames()
	for i, n := range names {
		if slices.Index(names, n) != i {
			t.Errorf("builtin %q registered twice", n)
		}
	}
}
