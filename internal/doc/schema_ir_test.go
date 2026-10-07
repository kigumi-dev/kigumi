package doc_test

import (
	"os"
	"path/filepath"
	"testing"

	"kigumi/internal/doc"
	"kigumi/internal/driver"
)

const schemaIRProbeSrc = `pub type Point = {
    pub x Int
    pub y Int
}

pub type Shape = Circle(r: Int) | Dot

/// Doubles n.
pub fn double(n: Int) -> Int {
    n * 2
}
`

func loadSchemaIRProbe(t *testing.T, src string) *driver.Module {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

const schemaIRProbeGolden = `{
  "envelope": {
    "format": "kigumi.schema",
    "major": 1,
    "minor": 0,
    "compiler": "0.1",
    "target": "x86_64-linux",
    "scope": [
      "main"
    ]
  },
  "declarations": [
    {
      "id": "main#type:Point",
      "package": "main",
      "kind": "type",
      "name": "Point",
      "visibility": "pub",
      "display": "pub type Point = {\n    pub x Int\n    pub y Int\n}",
      "fields": [
        {
          "id": "main#type:Point.x",
          "name": "x",
          "type": {
            "kind": "prim",
            "display": "Int",
            "prim": "Int"
          }
        },
        {
          "id": "main#type:Point.y",
          "name": "y",
          "type": {
            "kind": "prim",
            "display": "Int",
            "prim": "Int"
          }
        }
      ],
      "origin": {
        "file": "main.kg",
        "line": 1,
        "generated": false
      }
    },
    {
      "id": "main#type:Shape",
      "package": "main",
      "kind": "type",
      "name": "Shape",
      "visibility": "pub",
      "display": "pub type Shape = Circle(r: Int) | Dot",
      "variants": [
        {
          "id": "main#type:Shape.Circle",
          "name": "Circle",
          "payload": [
            {
              "id": "main#type:Shape.Circle.r",
              "name": "r",
              "type": {
                "kind": "prim",
                "display": "Int",
                "prim": "Int"
              },
              "copy": true
            }
          ]
        },
        {
          "id": "main#type:Shape.Dot",
          "name": "Dot"
        }
      ],
      "origin": {
        "file": "main.kg",
        "line": 6,
        "generated": false
      }
    },
    {
      "id": "main#fn:double",
      "package": "main",
      "kind": "fn",
      "name": "double",
      "visibility": "pub",
      "display": "pub fn double(n: Int) -\u003e Int",
      "doc": "Doubles n.",
      "params": [
        {
          "id": "main#fn:double.n",
          "name": "n",
          "type": {
            "kind": "prim",
            "display": "Int",
            "prim": "Int"
          }
        }
      ],
      "contracts": {
        "inferredPure": true,
        "inferredNoalloc": true,
        "operations": [
          "panic"
        ]
      },
      "origin": {
        "file": "main.kg",
        "line": 9,
        "generated": false
      }
    }
  ]
}`

// TestSchemaIRJSONGolden pins SchemaIRJSON's output for a small module; a
// change here means the Schema IR's shape or encoding moved, not a typo.
func TestSchemaIRJSONGolden(t *testing.T) {
	m := loadSchemaIRProbe(t, schemaIRProbeSrc)
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	out, err := doc.SchemaIRJSON(m, res, "", "x86_64-linux", "")
	if err != nil {
		t.Fatalf("SchemaIRJSON: %v", err)
	}
	if string(out) != schemaIRProbeGolden {
		t.Errorf("SchemaIRJSON output changed:\n%s", out)
	}
}

// TestSchemaIRJSONStable checks two runs over the same checked module agree
// byte for byte, the determinism a derive generator's cache key depends on.
func TestSchemaIRJSONStable(t *testing.T) {
	m := loadSchemaIRProbe(t, schemaIRProbeSrc)
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	first, err := doc.SchemaIRJSON(m, res, "", "x86_64-linux", "")
	if err != nil {
		t.Fatalf("first SchemaIRJSON: %v", err)
	}
	second, err := doc.SchemaIRJSON(m, res, "", "x86_64-linux", "")
	if err != nil {
		t.Fatalf("second SchemaIRJSON: %v", err)
	}
	if string(first) != string(second) {
		t.Errorf("SchemaIRJSON is not stable across runs:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}

// TestSchemaIRJSONErrorsOnUnencodableMetadata checks the error SchemaIRJSON
// returns rather than panicking: a metadata float that overflows float64 renders +Inf, which encoding/json refuses.
func TestSchemaIRJSONErrorsOnUnencodableMetadata(t *testing.T) {
	src := `@schema
pub type Meta = {
    v Float
}

pub fn huge(f: Float) -> Meta {
    Meta { v: f }
}

pub type Point = {
    pub x Int { huge(1e400) }
}
`
	m := loadSchemaIRProbe(t, src)
	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	if _, err := doc.SchemaIRJSON(m, res, "", "x86_64-linux", ""); err == nil {
		t.Fatal("expected an error for a +Inf metadata argument")
	}
}
