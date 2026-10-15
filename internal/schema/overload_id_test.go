package schema_test

import (
	"testing"

	"kigumi/internal/schema"
)

func TestOverloadsHaveDistinctIDs(t *testing.T) {
	m, res := loadModule(t, `
pub fn f(x: Int) -> Int { x }
pub fn f(x: String) -> String { x }
pub fn g(x: Int) -> Int { x }

`)
	mod, _ := schema.Build(m, res, schema.Options{})
	ids := map[string]string{}
	var fs int
	for _, d := range mod.Declarations {
		if d.Name != "f" && d.Name != "g" {
			continue
		}
		if prev, dup := ids[d.ID]; dup {
			t.Fatalf("duplicate id %q for %s and %s", d.ID, prev, d.Name)
		}
		ids[d.ID] = d.Name
		if d.Name == "f" {
			fs++
		}
	}
	if fs != 2 || len(ids) != 3 {
		t.Fatalf("want 2 overloads of f and 1 g, got %v", ids)
	}
	if _, ok := ids["main#fn:g"]; !ok {
		t.Fatalf("a lone declaration keeps its plain id: %v", ids)
	}
}
