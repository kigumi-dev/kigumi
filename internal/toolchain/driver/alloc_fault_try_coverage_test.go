package driver_test

import (
	"slices"
	"strings"
	"testing"

	opdef "kigumi/internal/compiler/mir/ops"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/devkit/fixture"
	"kigumi/internal/devkit/semfixture"
	"kigumi/internal/toolchain/contract/schema"
)

// oomInert are try functions with no allocation on their path, so no tick can be refused inside them.
var oomInert = map[string]string{
	"mem.trySecureErase":         "zeroes the region in place and builds no result of its own",
	"array.BoundedArray.tryPush": "fills a slot new already allocated; it fails only on a full array",
	"platform.tryWaitProcess":    "wraps waitpid's scalar status and allocates nothing",
	"proc.Process.tryWait":       "forwards the platform wait status and allocates nothing",
}

// TestEveryTryFunctionHasATwoCallSweep reads the OOM metadata from the
// manifest: `covers` names the std try functions a sweep drives, and `pair`
// names its second-call fixture. The manifest, not a Go table, is the one
// place the fault corpus is described.
func TestEveryTryFunctionHasATwoCallSweep(t *testing.T) {
	t.Parallel()
	res := sem.CheckWith(&sem.Module{Packages: semfixture.LoadStd(t, "../../../std")}, schema.GenerateDerive)
	var tries []string
	for id := range res.Entities {
		e := &res.Entities[id]
		if e.Kind != sem.EntFn || e.Flags&sem.EfStd == 0 || len(e.Name) < 4 || !strings.HasPrefix(e.Name, "try") || e.Name[3] < 'A' || e.Name[3] > 'Z' {
			continue
		}
		tries = append(tries, opdef.StdKey(res, sem.EntityID(id), true))
	}
	if len(tries) < 50 {
		t.Fatalf("found only %d try functions; std not loaded?", len(tries))
	}
	m := loadManifest(t)
	covers := map[string][]*fixture.Fixture{}
	for i := range m.Fixtures {
		fx := &m.Fixtures[i]
		if fx.Kind != "oom" {
			continue
		}
		for _, key := range fx.Covers {
			covers[key] = append(covers[key], fx)
		}
	}
	for _, key := range tries {
		if _, inert := oomInert[key]; inert {
			continue
		}
		entries := covers[key]
		if len(entries) == 0 {
			t.Errorf("%s has no fault sweep fixture; add an oom/<name>.kg pair and list `covers` on its manifest entry", key)
			continue
		}
		for _, fx := range entries {
			if fx.Pair == "" {
				t.Errorf("%s: manifest entry %s has no pair", key, fx.ID)
				continue
			}
			for _, id := range []string{fx.ID, fx.Pair} {
				if _, err := fixture.Find("../../..", id); err != nil {
					t.Errorf("%s: %v", key, err)
				}
			}
		}
	}
	for key := range covers {
		if !slices.Contains(tries, key) {
			t.Errorf("the manifest covers %s, which is no std try function", key)
		}
	}
}
