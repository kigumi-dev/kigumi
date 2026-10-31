package schema_test

import (
	"encoding/json"
	"testing"

	"kigumi/internal/toolchain/contract/schema"
)

func TestBuildCallableReferenceLifetime(t *testing.T) {
	m, res := loadModule(t, `pub fn use['a](named: fn(&'a String) -> Void, scoped: fn(&String) -> Void) {}`)
	mod, _ := schema.Build(m, res, schema.Options{})
	for _, d := range mod.Declarations {
		if d.Name != "use" {
			continue
		}
		encoded, err := json.Marshal(d.Params)
		if err != nil {
			t.Fatal(err)
		}
		var params []*schema.Member
		if err := json.Unmarshal(encoded, &params); err != nil {
			t.Fatal(err)
		}
		named, scoped := params[0].Type.Params[0], params[1].Type.Params[0]
		if named.Lifetime == nil || named.Lifetime.Kind != schema.TRParam || named.Lifetime.Display != "'a" {
			t.Fatalf("named reference lost its lifetime in JSON: %s", encoded)
		}
		if scoped.Lifetime != nil {
			t.Fatalf("scoped reference acquired a lifetime: %s", encoded)
		}
		return
	}
	t.Fatal("use declaration missing")
}
