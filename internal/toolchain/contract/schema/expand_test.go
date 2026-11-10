package schema_test

import (
	"encoding/json"
	"strings"
	"testing"

	"kigumi/internal/toolchain/contract/schema"
)

const expandProbeSrc = `
pub type Point = {
    pub x Int
    pub y Int
}

fn same(a: Point, b: Point) -> Bool {
    a == b
}
`

// TestGeneratedFiles lists a generated file with the derive request that
// produced it, the data `kigumi expand` prints.
func TestGeneratedFiles(t *testing.T) {
	m, res := loadModule(t, expandProbeSrc)
	_, byEntity := schema.Build(m, res, schema.Options{})
	files := schema.GeneratedFiles(res, byEntity)
	if len(files) != 1 {
		t.Fatalf("want one generated file, got %+v", files)
	}
	f := files[0]
	if f.Name != "proto.derived.kg" || f.Package != "main" {
		t.Errorf("file is %s/%s, want main/proto.derived.kg", f.Package, f.Name)
	}
	if !strings.Contains(f.Text, "fn Point.equals(") {
		t.Errorf("generated text is missing the equals method:\n%s", f.Text)
	}
	if f.ContentHash == "" {
		t.Errorf("generated file has no content hash")
	}
	if len(f.Sources) != 1 || f.Sources[0].Generator != "derive:Eq" {
		t.Fatalf("want one derive:Eq source, got %+v", f.Sources)
	}
	if f.Sources[0].Origin == nil || f.Sources[0].Origin.ID != "main#type:Point" {
		t.Errorf("source origin is %+v, want main#type:Point", f.Sources[0].Origin)
	}
}

// TestProjectDocumentJSON pins emit-schema's two mandatory version fields
// (language spec §13.5) next to the schema IR they describe.
func TestProjectDocumentJSON(t *testing.T) {
	m, res := loadModule(t, expandProbeSrc)
	mod, _ := schema.Build(m, res, schema.Options{Target: "x86_64-linux"})
	data, err := schema.ProjectDocumentJSON(mod)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		SchemaVersion       string `json:"schemaVersion"`
		CanonicalABIVersion string `json:"canonicalAbiVersion"`
		Envelope            struct {
			Format string `json:"format"`
			Minor  int    `json:"minor"`
		} `json:"envelope"`
		Declarations []struct {
			Name string `json:"name"`
		} `json:"declarations"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.SchemaVersion != "1.4" || doc.Envelope.Minor != 4 {
		t.Errorf("schemaVersion = %q, envelope minor = %d, want 1.4", doc.SchemaVersion, doc.Envelope.Minor)
	}
	if doc.CanonicalABIVersion != "0.3" {
		t.Errorf("canonicalAbiVersion = %q, want 0.3", doc.CanonicalABIVersion)
	}
	if doc.Envelope.Format != "kigumi.schema" {
		t.Errorf("envelope format = %q", doc.Envelope.Format)
	}
	if len(doc.Declarations) != 2 {
		t.Errorf("declarations = %+v, want Point and same", doc.Declarations)
	}
}
