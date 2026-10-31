package schema_test

import (
	"bytes"
	"testing"

	"kigumi/internal/backend/vm"
	"kigumi/internal/compiler/sem"
	"kigumi/internal/toolchain/contract/schema"
	"kigumi/internal/toolchain/driver"
)

const deriveParitySrc = `
import json from std/json

type Point = {
    pub x Int
    pub y Int
}

type Shape = Circle(r: Int) | Dot

type Wrap[T] = {
    pub inner T
}

type Pair[T] = {
    pub first T
    pub second T
}

type Tagged = {
    pub id Int { json.name("tag_id") }
    pub note String? { json.omitNone() }
}

fn useEq() -> Bool {
    Point { x: 1, y: 2 } == Point { x: 1, y: 2 }
        && Shape.Circle(1) == Shape.Circle(1)
        && Wrap { inner: Point { x: 1, y: 2 } } == Wrap { inner: Point { x: 1, y: 2 } }
}

let ok = useEq()
print "${ok} ${hash(&Point { x: 1, y: 2 })}"
let p = Pair { first: 1, second: 2 }
print json.encode(p)?
let back = json.decode[Pair[Int]](json.encode(p)?)?
print "${back.first + back.second}"
print json.encode(Point { x: 5, y: 6 })?
print json.encode(Shape.Circle(3))?
print json.encode(Tagged { id: 1, note: None })?
let tagged = json.decode[Tagged](json.encode(Tagged { id: 2, note: Some("hi") })?)?
print "${tagged.id} ${tagged.note || "?"}"
`

func runParityModule(t *testing.T, gen func(*driver.Module) (*driver.Analysis, error)) (string, string) {
	t.Helper()
	m, _ := loadModule(t, deriveParitySrc)
	a, err := gen(m)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	if a.Res.HasErrors() {
		t.Fatalf("unexpected errors:\n%s", a.Res.Render())
	}
	var sources bytes.Buffer
	for _, g := range a.Res.GeneratedSources() {
		sources.WriteString(g.Package + "/" + g.Name + ":\n" + g.Text)
	}
	var stdout, stderr bytes.Buffer
	if ok, why := vm.Supports(a.Prog); !ok {
		t.Fatalf("vm refuses program: %s", why)
	}
	if _, err := vm.Run(a.Prog, vm.Options{}, &stdout, &stderr); err != nil {
		t.Fatalf("vm run: %v (stderr: %s)", err, stderr.String())
	}
	return sources.String(), stdout.String()
}

func TestDeriveGeneratorParity(t *testing.T) {
	legacySrc, legacyOut := runParityModule(t, driver.Analyze)
	newSrc, newOut := runParityModule(t, func(m *driver.Module) (*driver.Analysis, error) {
		return driver.AnalyzeWith(m, schema.GenerateDerive)
	})
	if legacySrc != newSrc {
		t.Errorf("generated source differs:\n--- legacy ---\n%s\n--- schema ---\n%s", legacySrc, newSrc)
	}
	if legacyOut != newOut {
		t.Errorf("VM stdout differs:\nlegacy: %q\nschema: %q", legacyOut, newOut)
	}
	if legacyOut == "" {
		t.Fatal("expected non-empty stdout")
	}
}

func TestGenerateDeriveDeterministic(t *testing.T) {
	m, res := loadModule(t, deriveParitySrc)
	_, byEntity := schema.Build(m, res, schema.Options{})
	var point sem.EntityID
	for ent, d := range byEntity {
		if d.Name == "Point" {
			point = ent
		}
	}
	if point == 0 {
		t.Fatal("Point not found in Schema IR")
	}
	req := sem.DeriveRequest{Decl: point, Interface: "Eq"}
	first, err := schema.GenerateDerive(res, req)
	if err != nil {
		t.Fatalf("first generate: %v", err)
	}
	second, err := schema.GenerateDerive(res, req)
	if err != nil {
		t.Fatalf("second generate: %v", err)
	}
	if first != second {
		t.Errorf("non-deterministic output:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
	if first == "" {
		t.Fatal("expected non-empty generated text")
	}
}

// TestGenerateDeriveUnsupportedInterface checks the error carries a DiagKind
// sem can map to a diagnostic code without importing this package.
func TestGenerateDeriveUnsupportedInterface(t *testing.T) {
	m, res := loadModule(t, deriveParitySrc)
	_, byEntity := schema.Build(m, res, schema.Options{})
	var point sem.EntityID
	for ent, d := range byEntity {
		if d.Name == "Point" {
			point = ent
		}
	}
	_, err := schema.GenerateDerive(res, sem.DeriveRequest{Decl: point, Interface: "Ord"})
	if err == nil {
		t.Fatal("expected an error for an unsupported interface")
	}
	kind, ok := err.(interface{ DiagKind() string })
	if !ok || kind.DiagKind() != "unsupported" {
		t.Errorf("expected DiagKind \"unsupported\", got %v (ok=%v)", err, ok)
	}
}

// TestGenerateDeriveDisplay reads only Declaration.Name/Visibility, so the
// expected text is fixed regardless of Point's fields.
func TestGenerateDeriveDisplay(t *testing.T) {
	m, res := loadModule(t, deriveParitySrc)
	_, byEntity := schema.Build(m, res, schema.Options{})
	var point sem.EntityID
	for ent, d := range byEntity {
		if d.Name == "Point" {
			point = ent
		}
	}
	if point == 0 {
		t.Fatal("Point not found in Schema IR")
	}
	req := sem.DeriveRequest{Decl: point, Interface: "Display"}
	first, err := schema.GenerateDerive(res, req)
	if err != nil {
		t.Fatalf("first generate: %v", err)
	}
	want := "pure fn Point.display(self) -> String {\n    debug(&self)\n}\n"
	if first != want {
		t.Errorf("got:\n%s\nwant:\n%s", first, want)
	}
	second, err := schema.GenerateDerive(res, req)
	if err != nil {
		t.Fatalf("second generate: %v", err)
	}
	if first != second {
		t.Errorf("non-deterministic output:\n--- first ---\n%s\n--- second ---\n%s", first, second)
	}
}
