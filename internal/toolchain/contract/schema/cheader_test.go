package schema_test

import (
	"strings"
	"testing"

	"kigumi/internal/toolchain/contract/schema"
)

const headerSrc = `
extern(C) {
    type Handle
    fn kg_test_open(path: *const u8, flags: i32, ...) -> i32
    fn shutdown(fd: i32) -> Void
}

pub type Point layout(C) = {
    pub x i32
    pub y i32
}

pub type Segment layout(C) = {
    pub from Point
    pub to Point
}

export(C) {
    fn origin() -> Point {
        Point { x: 0, y: 0 }
    }
}
`

// TestEmitCHeader renders the C boundary: opaque extern types, layout(C)
// records in dependency order, and extern/export(C) prototypes.
func TestEmitCHeader(t *testing.T) {
	m, res := loadModule(t, headerSrc)
	mod, _ := schema.Build(m, res, schema.Options{})
	h := schema.EmitCHeader(mod, schema.CHeaderOptions{})
	if len(h.Diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v", h.Diagnostics)
	}
	for _, want := range []string{
		"#ifndef KIGUMI_MAIN_H",
		"#include <stdint.h>",
		"typedef struct Handle Handle;",
		"struct Point {\n    int32_t x;\n    int32_t y;\n};",
		"typedef struct Point Point;",
		"struct Segment {\n    struct Point from;\n    struct Point to;\n};",
		"int32_t kg_test_open(const uint8_t *path, int32_t flags, ...);",
		"void shutdown(int32_t fd);",
		"struct Point origin(void);",
	} {
		if !strings.Contains(h.Text, want) {
			t.Errorf("missing %q in:\n%s", want, h.Text)
		}
	}
	if strings.Index(h.Text, "struct Point {") > strings.Index(h.Text, "struct Segment {") {
		t.Errorf("a by-value field's struct must be complete before its holder:\n%s", h.Text)
	}
	if len(h.Declarations) != 6 {
		t.Errorf("emitted %d declarations, want 6: %+v", len(h.Declarations), h.Declarations)
	}
}

// TestEmitCHeaderStable pins determinism: the same checked module renders the
// same header, the property a build cache key depends on.
func TestEmitCHeaderStable(t *testing.T) {
	m, res := loadModule(t, headerSrc)
	mod, _ := schema.Build(m, res, schema.Options{})
	first := schema.EmitCHeader(mod, schema.CHeaderOptions{Guard: "API_H"})
	second := schema.EmitCHeader(mod, schema.CHeaderOptions{Guard: "API_H"})
	if first.Text != second.Text {
		t.Errorf("header is not stable across runs")
	}
	if !strings.Contains(first.Text, "#ifndef API_H") {
		t.Errorf("--guard was not used:\n%s", first.Text)
	}
}

// TestEmitCHeaderSkipsNonCDeclarations checks a declaration the header cannot
// express is reported, never silently dropped.
func TestEmitCHeaderSkipsNonCDeclarations(t *testing.T) {
	src := `
extern(C) {
    static errno: i32
    fn kg_test_probe(path: *const u8) -> i32
}
`
	m, res := loadModule(t, src)
	mod, _ := schema.Build(m, res, schema.Options{})
	h := schema.EmitCHeader(mod, schema.CHeaderOptions{})
	if len(h.Diagnostics) != 1 || h.Diagnostics[0].Code != schema.DiagCHeaderUnsupportedDecl {
		t.Fatalf("want one unsupported-declaration diagnostic, got %+v", h.Diagnostics)
	}
	if h.Diagnostics[0].Source == "" {
		t.Errorf("a diagnostic must name the declaration it dropped")
	}
	if !strings.Contains(h.Text, "int32_t kg_test_probe(const uint8_t *path);") {
		t.Errorf("the supported declaration was dropped too:\n%s", h.Text)
	}
}

// TestEmitCHeaderABIBlockTypesOnlyWalked checks the IR sees declarations
// nested in an ABI block, which a top-level walk alone would miss.
func TestSchemaIRIncludesABIDeclarations(t *testing.T) {
	m, res := loadModule(t, headerSrc)
	mod, _ := schema.Build(m, res, schema.Options{})
	var open *schema.Declaration
	for _, d := range mod.Declarations {
		if d.Name == "kg_test_open" {
			open = d
		}
	}
	if open == nil {
		t.Fatalf("extern(C) fn is missing from the Schema IR")
	}
	if open.ABI == nil || open.ABI.Block != "extern" || open.ABI.Name != "C" || !open.ABI.Variadic {
		t.Errorf("open's ABI is %+v, want extern(C) variadic", open.ABI)
	}
	if open.Return == nil || open.Return.Prim != "i32" {
		t.Errorf("open's return type is %+v, want i32", open.Return)
	}
}
