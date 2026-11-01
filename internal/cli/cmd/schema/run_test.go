package schema_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

// TestSchema lists declarations with their metadata attributes.
func TestSchema(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "web"), 0o755)
	os.WriteFile(filepath.Join(root, "mod.kg"), clitest.Manifest("app", ""), 0o644)
	os.WriteFile(filepath.Join(root, "web", "web.kg"), []byte("@schema\npub type Metadata = {\n    tag String\n}\n\npub pure fn get(path: String) -> Metadata {\n    Metadata { tag: path }\n}\n\npub pure fn param(name: String) -> Metadata {\n    Metadata { tag: name }\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("import web from app/web\n\n@web.get(\"/users/:id\")\nfn show(@web.param(\"id\") id: Int) -> Int {\n    id\n}\n\nfn helper() -> Int {\n    1\n}\n\nprint \"${show(1)} ${helper()}\"\n"), 0o644)
	std, _ := filepath.Abs("../../../../std")
	code, out, errOut := clitest.Execute(t, "--std", std, "schema", "--attr", "web.get", root)
	if code != 0 {
		t.Fatalf("schema: %s", errOut)
	}
	for _, want := range []string{"\"name\": \"show\"", "\"name\": \"web.get\"", "\"/users/:id\"", "\"name\": \"web.param\""} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "helper") {
		t.Errorf("--attr should filter out undecorated declarations:\n%s", out)
	}
}

// TestSchemaIR lists the versioned typed Schema IR projection (--ir).
func TestSchemaIR(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "web"), 0o755)
	os.WriteFile(filepath.Join(root, "mod.kg"), clitest.Manifest("app", ""), 0o644)
	os.WriteFile(filepath.Join(root, "web", "web.kg"), []byte("@schema\npub type Metadata = {\n    tag String\n}\n\npub pure fn get(path: String) -> Metadata {\n    Metadata { tag: path }\n}\n\npub pure fn param(name: String) -> Metadata {\n    Metadata { tag: name }\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("import web from app/web\n\n@web.get(\"/users/:id\")\nfn show(@web.param(\"id\") id: Int) -> Int {\n    id\n}\n"), 0o644)
	std, _ := filepath.Abs("../../../../std")
	code, out, errOut := clitest.Execute(t, "--std", std, "--target", "x86_64-linux", "schema", "--ir", root)
	if code != 0 {
		t.Fatalf("schema --ir: %s", errOut)
	}
	for _, want := range []string{"\"format\": \"kigumi.schema\"", "\"name\": \"show\"", "\"schemaId\"", "\"name\": \"web.get\"", "\"id\": \"app#fn:show\"", "\"target\": \"x86_64-linux\""} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s in:\n%s", want, out)
		}
	}
}
