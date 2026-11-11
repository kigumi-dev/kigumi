package sem_test

import (
	"sort"
	"strings"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/contract/schema"
)

func TestWitnessPrefersInterfacePackage(t *testing.T) {
	fx := parseFixture(t, semFixture(t, "witness_interface_package"))
	res := sem.CheckWith(buildModule(t, fx), schema.GenerateDerive)
	got := map[string][]string{}
	for _, f := range res.Files {
		if f.Tree == nil || !strings.HasPrefix(f.Tree.File.Name, "main/") {
			continue
		}
		for _, call := range f.Calls {
			for _, p := range call.Passes {
				if e := res.Entities[p.Ent]; p.Kind == sem.WitnessFn && e.Name == "show" {
					got[f.Tree.File.Name] = append(got[f.Tree.File.Name], res.Packages[e.Pkg].Path)
				}
			}
		}
	}
	for k := range got {
		sort.Strings(got[k])
	}
	want := map[string]string{"main/main.kg": "std/pb", "main/other.kg": "std/pa", "main/both.kg": "std/pa,std/pb"}
	for name, w := range want {
		if g := strings.Join(got[name], ","); g != w {
			t.Errorf("%s: show witnesses from %q, want %q", name, g, w)
		}
	}
}
