package llgen_test

import (
	"strings"
	"testing"

	"kigumi/internal/compiler/mir"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/toolchain/contract/schema"
)

// TestRegistryMatchesStd holds the registry and the bodiless std functions to each other: every key names one, and every one resolves to an op or is lowered at the call site.
func TestRegistryMatchesStd(t *testing.T) {
	res := sem.CheckWith(&sem.Module{Packages: testkit.LoadStd(t, "../../../std")}, schema.GenerateDerive)
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	bodiless := map[string]bool{}
	for id := range res.Entities {
		if key, ok := primitiveKey(res, sem.EntityID(id)); ok {
			bodiless[strings.TrimPrefix(key, "std/")] = true
		}
	}
	if len(bodiless) < 40 {
		t.Fatalf("only %d bodiless std functions found; std not loaded as std?", len(bodiless))
	}
	for _, op := range mir.Registry.Ops {
		if op.Retired {
			continue
		}
		if len(op.Keys)+len(op.Suffixes)+len(op.Prefixes) == 0 {
			t.Errorf("op %s has no key, suffix or prefix, so no std function can resolve to it", op.Name)
		}
		for _, key := range op.Keys {
			if !bodiless[key] {
				t.Errorf("op %s: key %s is not a bodiless std function", op.Name, key)
			}
		}
	}
	for key := range bodiless {
		if _, ok := mir.StdOpFor(key); !ok && !lowered[key] && !conversionKey(key) {
			t.Errorf("%s: bodiless std function that resolves to no registry op", key)
		}
	}
}
