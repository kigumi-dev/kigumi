package schema_test

import (
	"encoding/json"
	"testing"

	"kigumi/internal/toolchain/contract/schema"
)

const protoDeriveSrc = `
type Point = {
    pub x Int
    pub y Int
}

fn useEq(a: Point, b: Point) -> Bool {
    a == b
}
`

func TestProvenanceEqDerive(t *testing.T) {
	m, res := loadModule(t, protoDeriveSrc)
	_, byEntity := schema.Build(m, res, schema.Options{})
	prov := schema.BuildProvenance(res, byEntity)
	if len(prov) == 0 {
		t.Fatal("expected a generated Eq artifact")
	}
	var got *schema.GeneratedArtifact
	for i := range prov {
		if prov[i].Generator == "derive:Eq" {
			got = &prov[i]
		}
	}
	if got == nil {
		t.Fatalf("no derive:Eq artifact in %+v", prov)
	}
	if got.Origin == nil || got.Origin.Name != "Point" {
		t.Fatalf("unexpected origin: %+v", got.Origin)
	}
	if got.InputDigest == "" || got.ContentHash == "" {
		t.Fatalf("expected non-empty digests: %+v", got)
	}
}

func TestProvenanceDeterministic(t *testing.T) {
	m1, res1 := loadModule(t, protoDeriveSrc)
	_, by1 := schema.Build(m1, res1, schema.Options{})
	prov1 := schema.BuildProvenance(res1, by1)

	m2, res2 := loadModule(t, protoDeriveSrc)
	_, by2 := schema.Build(m2, res2, schema.Options{})
	prov2 := schema.BuildProvenance(res2, by2)

	if len(prov1) != len(prov2) || len(prov1) == 0 {
		t.Fatalf("expected matching non-empty provenance, got %d and %d", len(prov1), len(prov2))
	}
	j1, err := json.Marshal(prov1)
	if err != nil {
		t.Fatal(err)
	}
	j2, err := json.Marshal(prov2)
	if err != nil {
		t.Fatal(err)
	}
	if string(j1) != string(j2) {
		t.Fatalf("non-deterministic provenance:\n%s\n%s", j1, j2)
	}
}

const jsonDeriveSrc = `
import json from std/json

type Author = {
    pub name String
}

fn useJson(a: Author) -> String! {
    json.encode(a)
}
`

func TestProvenanceJSONDerive(t *testing.T) {
	m, res := loadModule(t, jsonDeriveSrc)
	_, byEntity := schema.Build(m, res, schema.Options{})
	prov := schema.BuildProvenance(res, byEntity)
	var sawEncode bool
	for _, p := range prov {
		if p.Generator == "derive:Encode" {
			sawEncode = true
			if p.File != "json.derived.kg" {
				t.Errorf("unexpected file: %s", p.File)
			}
		}
	}
	if !sawEncode {
		t.Fatalf("expected a derive:Encode artifact in %+v", prov)
	}
}

func TestProvenanceDigestChangesWithInput(t *testing.T) {
	m1, res1 := loadModule(t, protoDeriveSrc)
	_, by1 := schema.Build(m1, res1, schema.Options{})
	prov1 := schema.BuildProvenance(res1, by1)

	changed := `
type Point = {
    pub x Int
    pub y Int
    pub z Int
}

fn useEq(a: Point, b: Point) -> Bool {
    a == b
}
`
	m2, res2 := loadModule(t, changed)
	_, by2 := schema.Build(m2, res2, schema.Options{})
	prov2 := schema.BuildProvenance(res2, by2)

	if len(prov1) == 0 || len(prov2) == 0 {
		t.Fatal("expected generated artifacts in both modules")
	}
	if prov1[0].InputDigest == prov2[0].InputDigest {
		t.Fatalf("expected InputDigest to change when the type's fields change")
	}
	if prov1[0].ContentHash == prov2[0].ContentHash {
		t.Fatalf("expected ContentHash to change when the generated source changes")
	}
}
