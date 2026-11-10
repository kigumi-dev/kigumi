package emit_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

const emitSrc = `
extern(C) {
    type Handle
    fn kg_test_open(path: *const u8, flags: i32, ...) -> i32
}

pub type Point layout(C) = {
    pub x i32
    pub y i32
}

export(C) {
    fn origin() -> Point {
        Point { x: 0, y: 0 }
    }
}
`

func writeModule(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "mod.kg"), clitest.Manifest("app", ""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte(emitSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

func stdRoot(t *testing.T) string {
	t.Helper()
	std, err := filepath.Abs("../../../../std")
	if err != nil {
		t.Fatal(err)
	}
	return std
}

// TestEmitCHeader writes C declarations for the ABI boundary.
func TestEmitCHeader(t *testing.T) {
	root := writeModule(t)
	code, out, errOut := clitest.Execute(t, "--std", stdRoot(t), "emit-c-header", root)
	if code != 0 {
		t.Fatalf("emit-c-header: %s", errOut)
	}
	for _, want := range []string{
		"#ifndef KIGUMI_APP_H",
		"#include <stdint.h>",
		"typedef struct Handle Handle;",
		"struct Point {\n    int32_t x;\n    int32_t y;\n};",
		"int32_t kg_test_open(const uint8_t *path, int32_t flags, ...);",
		"struct Point origin(void);",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

// TestEmitCHeaderJSON reports what was declared and what was skipped.
func TestEmitCHeaderJSON(t *testing.T) {
	root := writeModule(t)
	code, out, errOut := clitest.Execute(t, "--std", stdRoot(t), "emit-c-header", "--format", "json", "--guard", "API_H", root)
	if code != 0 {
		t.Fatalf("emit-c-header --format json: %s", errOut)
	}
	var envelope struct {
		FormatVersion int    `json:"formatVersion"`
		Kind          string `json:"kind"`
		Data          struct {
			Guard        string `json:"guard"`
			Text         string `json:"text"`
			Declarations []struct {
				ID     string `json:"id"`
				Kind   string `json:"kind"`
				Symbol string `json:"symbol"`
			} `json:"declarations"`
			Diagnostics []struct {
				Code string `json:"code"`
			} `json:"diagnostics"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("emit-c-header JSON: %v\n%s", err, out)
	}
	if envelope.Kind != "emit-c-header" || envelope.FormatVersion != 1 {
		t.Errorf("envelope = %+v, want kind emit-c-header formatVersion 1", envelope)
	}
	if envelope.Data.Guard != "API_H" {
		t.Errorf("guard = %q, want API_H", envelope.Data.Guard)
	}
	if len(envelope.Data.Diagnostics) != 0 {
		t.Errorf("diagnostics = %+v, want none", envelope.Data.Diagnostics)
	}
	if len(envelope.Data.Declarations) != 4 {
		t.Errorf("declarations = %+v, want Handle, Point and two functions", envelope.Data.Declarations)
	}
	if !strings.Contains(envelope.Data.Text, "#ifndef API_H") {
		t.Errorf("report text ignores --guard:\n%s", envelope.Data.Text)
	}
}

// TestEmitCHeaderOutput writes the header to a file instead of stdout.
func TestEmitCHeaderOutput(t *testing.T) {
	root := writeModule(t)
	out := filepath.Join(t.TempDir(), "include", "app.h")
	code, stdout, errOut := clitest.Execute(t, "--std", stdRoot(t), "emit-c-header", "-o", out, root)
	if code != 0 {
		t.Fatalf("emit-c-header -o: %s", errOut)
	}
	if stdout != "" {
		t.Errorf("-o still wrote to stdout: %s", stdout)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "typedef struct Handle Handle;") {
		t.Errorf("written header is missing the opaque type:\n%s", data)
	}
}

// TestEmitSchema writes the canonical schema document with the version fields
// §13.5 requires and the C boundary declarations the header emitter reads.
func TestEmitSchema(t *testing.T) {
	root := writeModule(t)
	code, out, errOut := clitest.Execute(t, "--std", stdRoot(t), "--target", "x86_64-linux", "emit-schema", root)
	if code != 0 {
		t.Fatalf("emit-schema: %s", errOut)
	}
	var doc struct {
		SchemaVersion       string `json:"schemaVersion"`
		CanonicalABIVersion string `json:"canonicalAbiVersion"`
		Declarations        []struct {
			Name string `json:"name"`
			Kind string `json:"kind"`
			ABI  *struct {
				Block    string `json:"block"`
				Name     string `json:"name"`
				Variadic bool   `json:"variadic"`
			} `json:"abi"`
			Return *struct {
				Prim string `json:"prim"`
			} `json:"return"`
		} `json:"declarations"`
	}
	if err := json.Unmarshal([]byte(out), &doc); err != nil {
		t.Fatalf("emit-schema JSON: %v\n%s", err, out)
	}
	if doc.SchemaVersion != "1.4" || doc.CanonicalABIVersion != "0.3" {
		t.Errorf("versions = %q/%q, want 1.4/0.3", doc.SchemaVersion, doc.CanonicalABIVersion)
	}
	var open bool
	for _, d := range doc.Declarations {
		if d.Name != "kg_test_open" {
			continue
		}
		open = true
		if d.ABI == nil || d.ABI.Block != "extern" || d.ABI.Name != "C" || !d.ABI.Variadic {
			t.Errorf("kg_test_open ABI = %+v, want extern(C) variadic", d.ABI)
		}
		if d.Return == nil || d.Return.Prim != "i32" {
			t.Errorf("kg_test_open return = %+v, want i32", d.Return)
		}
	}
	if !open {
		t.Errorf("emit-schema dropped the extern(C) declaration:\n%s", out)
	}
}

// TestEmitSchemaOutput writes the document to a file.
func TestEmitSchemaOutput(t *testing.T) {
	root := writeModule(t)
	out := filepath.Join(t.TempDir(), "schema.json")
	code, stdout, errOut := clitest.Execute(t, "--std", stdRoot(t), "emit-schema", "-o", out, root)
	if code != 0 {
		t.Fatalf("emit-schema -o: %s", errOut)
	}
	if stdout != "" {
		t.Errorf("-o still wrote to stdout: %s", stdout)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\"canonicalAbiVersion\": \"0.3\"") {
		t.Errorf("written schema is missing the canonical ABI version:\n%s", data)
	}
}

// TestEmitSchemaAttr keeps --attr from being a dead flag: only declarations
// carrying the named attribute survive.
func TestEmitSchemaAttr(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "mod.kg"), clitest.Manifest("app", ""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "web", "web.kg"), []byte("@schema\npub type Meta = {\n    tag String\n}\n\npub pure fn get(path: String) -> Meta {\n    Meta { tag: path }\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	src := "import web from app/web\n\n@web.get(\"/x\")\nfn annotated() -> Int {\n    1\n}\n\n" + emitSrc
	if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	code, out, errOut := clitest.Execute(t, "--std", stdRoot(t), "emit-schema", "--attr", "web.get", root)
	if code != 0 {
		t.Fatalf("emit-schema --attr: %s", errOut)
	}
	if !strings.Contains(out, "\"name\": \"annotated\"") {
		t.Errorf("--attr web.get dropped the declaration carrying it:\n%s", out)
	}
	if strings.Contains(out, "\"name\": \"kg_test_open\"") {
		t.Errorf("--attr web.get kept an undecorated declaration:\n%s", out)
	}
}

// TestEmitSchemaAllowErrors emits declarations for a module whose body has a
// type error only when --allow-errors says so.
func TestEmitSchemaAllowErrors(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "mod.kg"), clitest.Manifest("app", ""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte("pub type Point layout(C) = {\n    pub x i32\n}\n\nfn broken() -> Int {\n    \"x\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	code, _, _ := clitest.Execute(t, "--std", stdRoot(t), "emit-schema", root)
	if code != 1 {
		t.Fatalf("emit-schema exit = %d, want 1 without --allow-errors", code)
	}
	code, out, errOut := clitest.Execute(t, "--std", stdRoot(t), "emit-schema", "--allow-errors", root)
	if code != 0 {
		t.Fatalf("emit-schema --allow-errors: %s", errOut)
	}
	if !strings.Contains(out, "\"name\": \"Point\"") {
		t.Errorf("--allow-errors dropped the parsed declaration:\n%s", out)
	}
}
