package sem_test

import (
	"slices"
	"testing"

	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/contract/schema"
)

func TestCallableReferenceQualifierSubstitution(t *testing.T) {
	fx := fixture{sections: []section{{name: "src", body: `pub fn qualified['a, 'b](x: &'a String, y: &'b String) {}`}}}
	r := sem.CheckWith(buildModule(t, fx), schema.GenerateDerive)
	if r.HasErrors() {
		t.Fatal(r.Render())
	}
	var params []sem.EntityID
	for i, e := range r.Entities {
		if e.Kind == sem.EntFn && e.Name == "qualified" {
			params = r.Fn(sem.EntityID(i)).TypeParams
		}
	}
	if len(params) != 2 {
		t.Fatalf("lifetime parameters = %v", params)
	}
	tt := r.Types
	a, b := tt.Param(params[0]), tt.Param(params[1])
	plain := tt.Ref(sem.TyString, false)
	qualified := tt.WithRefLifetime(plain, a)
	if plain == qualified || qualified != tt.WithRefLifetime(plain, a) || r.TypeKey(plain) == r.TypeKey(qualified) {
		t.Fatal("reference contracts lost their distinct, stable identity")
	}
	sig := tt.Fn([]sem.TypeID{qualified}, sem.TyUnit, 0, false)
	inst := tt.Subst(sig, map[sem.EntityID]sem.TypeID{params[0]: b})
	if tt.RefLifetime(tt.Node(inst).Args[0]) != b || !tt.Contains(inst, b) || tt.Contains(inst, a) {
		t.Fatal("substitution lost or retained the wrong reference lifetime")
	}
	if got := tt.Params(inst); !slices.Equal(got, []sem.EntityID{params[1]}) {
		t.Fatalf("qualified signature parameters = %v", got)
	}
	if tt.ExactRefParams(sig, tt.Fn([]sem.TypeID{plain}, sem.TyUnit, 0, false)) {
		t.Fatal("a named reference unified with a scoped reference")
	}
}
