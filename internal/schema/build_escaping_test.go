package schema_test

import (
	"encoding/json"
	"strings"
	"testing"

	"kigumi/internal/schema"
)

const escapingSrc = `
pub type Slot[F] = { pub cb F }

pub fn keepFn(cb: escaping fn() -> Unit, n: Int) -> fn() -> Unit { cb }

pub fn keepBound[F: Fn[fn() -> Unit]](cb: escaping F) -> Slot[F] { Slot { cb: cb } }

pub fn callFn(cb: fn() -> Unit, tail: ...escaping fn() -> Unit) -> Int { cb()
    0 }

pub fn callBound[F: Fn[fn() -> Unit]](cb: F) -> Unit { cb() }

pub fn lambdaOnly() -> fn(fn() -> Unit) -> Unit { (f) => f() }
`

func TestBuildParamEscaping(t *testing.T) {
	m, res := loadModule(t, escapingSrc)
	mod, _ := schema.Build(m, res, schema.Options{})
	want := map[string][]bool{
		"keepFn":    {true, false},
		"keepBound": {true},
		"callFn":    {false, true},
		"callBound": {false},
	}
	seen := 0
	for _, d := range mod.Declarations {
		flags, ok := want[d.Name]
		if !ok {
			continue
		}
		seen++
		if len(d.Params) != len(flags) {
			t.Fatalf("%s: %d params, want %d", d.Name, len(d.Params), len(flags))
		}
		for i, p := range d.Params {
			if p.Escaping != flags[i] {
				t.Errorf("%s param %s: Escaping = %v, want %v", d.Name, p.Name, p.Escaping, flags[i])
			}
		}
	}
	if seen != len(want) {
		t.Fatalf("saw %d of %d declarations", seen, len(want))
	}
}

func TestBuildParamEscapingJSON(t *testing.T) {
	m, res := loadModule(t, escapingSrc)
	mod, _ := schema.Build(m, res, schema.Options{})
	out, err := json.Marshal(mod)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, `"escaping":true`) {
		t.Fatalf("no escaping field in %s", text)
	}
	if !strings.Contains(text, `"minor":1`) {
		t.Fatalf("envelope minor not bumped in %s", text)
	}
	if strings.Count(text, `"escaping"`) != 3 {
		t.Fatalf("want escaping on exactly three params, got %d in %s", strings.Count(text, `"escaping"`), text)
	}
}
