package schema_test

import (
	"encoding/json"
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

// TestSchemaAllowErrorsUnresolved checks `--allow-errors` never prints a
// signature full of `@...` next to an empty attributes list: metadata the
// checker could not resolve is listed separately.
func TestSchemaAllowErrorsUnresolved(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "web"), 0o755)
	os.WriteFile(filepath.Join(root, "mod.kg"), clitest.Manifest("app", ""), 0o644)
	// web.get returns a type without `@schema`, so E254 makes it unresolvable.
	os.WriteFile(filepath.Join(root, "web", "web.kg"), []byte("pub type Metadata = {\n    tag String\n}\n\npub pure fn get(path: String) -> Metadata {\n    Metadata { tag: path }\n}\n\npub type Row = {\n    id Int {\n        get(\"x\")\n    }\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("import web from app/web\n\n@web.get(\"/users/:id\")\nfn show() -> Int {\n    1\n}\n\nprint \"${show()}\"\n"), 0o644)
	std, _ := filepath.Abs("../../../../std")

	type declaration struct {
		Name       string `json:"name"`
		Signature  string `json:"signature"`
		Attributes []struct {
			Name string `json:"name"`
		} `json:"attributes"`
		UnresolvedAttributes []string `json:"unresolvedAttributes"`
		Fields               []struct {
			Name                 string   `json:"name"`
			UnresolvedAttributes []string `json:"unresolvedAttributes"`
		} `json:"fields"`
	}
	parse := func(t *testing.T, out string) map[string]declaration {
		t.Helper()
		var envelope struct {
			Declarations []declaration `json:"declarations"`
		}
		if err := json.Unmarshal([]byte(out), &envelope); err != nil {
			t.Fatalf("schema --allow-errors output did not parse: %v\n%s", err, out)
		}
		byName := map[string]declaration{}
		for _, d := range envelope.Declarations {
			byName[d.Name] = d
		}
		return byName
	}

	code, out, errOut := clitest.Execute(t, "--std", std, "schema", "--allow-errors", root)
	if code != 0 {
		t.Fatalf("schema --allow-errors: exit %d, stderr %s", code, errOut)
	}
	if !strings.Contains(errOut, "E254") {
		t.Errorf("schema --allow-errors should still report E254 on stderr:\n%s", errOut)
	}
	byName := parse(t, out)
	show, ok := byName["show"]
	if !ok {
		t.Fatalf("show missing from declarations:\n%s", out)
	}
	if !strings.Contains(show.Signature, "@web.get") {
		t.Errorf("show signature = %q, want the source attribute kept", show.Signature)
	}
	if len(show.Attributes) != 0 {
		t.Errorf("show attributes = %+v, want none (web.get is not a schema)", show.Attributes)
	}
	if len(show.UnresolvedAttributes) != 1 || show.UnresolvedAttributes[0] != "web.get" {
		t.Errorf("show unresolvedAttributes = %v, want [web.get]", show.UnresolvedAttributes)
	}
	row, ok := byName["Row"]
	if !ok || len(row.Fields) != 1 {
		t.Fatalf("Row/field missing from declarations:\n%s", out)
	}
	if len(row.Fields[0].UnresolvedAttributes) != 1 || row.Fields[0].UnresolvedAttributes[0] != "get" {
		t.Errorf("Row.id unresolvedAttributes = %v, want [get]", row.Fields[0].UnresolvedAttributes)
	}

	// The typed IR marks the same item unresolved instead of leaving a
	// metadata entry that looks like a resolved schema with no identity.
	code, out, errOut = clitest.Execute(t, "--std", std, "--target", "x86_64-linux", "schema", "--ir", "--allow-errors", root)
	if code != 0 {
		t.Fatalf("schema --ir --allow-errors: exit %d, stderr %s", code, errOut)
	}
	var ir struct {
		Declarations []struct {
			Name     string `json:"name"`
			Metadata []struct {
				Name       string `json:"name"`
				Unresolved bool   `json:"unresolved"`
			} `json:"metadata"`
		} `json:"declarations"`
	}
	if err := json.Unmarshal([]byte(out), &ir); err != nil {
		t.Fatalf("schema --ir output did not parse: %v\n%s", err, out)
	}
	foundIR := false
	for _, d := range ir.Declarations {
		if d.Name != "show" {
			continue
		}
		for _, m := range d.Metadata {
			if m.Name == "web.get" && m.Unresolved {
				foundIR = true
			}
		}
	}
	if !foundIR {
		t.Errorf("schema --ir did not mark web.get unresolved:\n%s", out)
	}

	// --attr still finds a declaration when the requested attribute is the
	// one that could not be resolved.
	code, out, errOut = clitest.Execute(t, "--std", std, "schema", "--allow-errors", "--attr", "web.get", root)
	if code != 0 {
		t.Fatalf("schema --allow-errors --attr: exit %d, stderr %s", code, errOut)
	}
	byName = parse(t, out)
	if _, ok := byName["show"]; !ok {
		t.Errorf("--attr web.get dropped the declaration that carries it:\n%s", out)
	}
	if _, ok := byName["Row"]; ok {
		t.Errorf("--attr web.get kept an unrelated declaration:\n%s", out)
	}
}

// TestSchemaResolvedMetadataUnchanged checks a resolvable attribute still
// appears as a plain attribute with no unresolved list.
func TestSchemaResolvedMetadataUnchanged(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "web"), 0o755)
	os.WriteFile(filepath.Join(root, "mod.kg"), clitest.Manifest("app", ""), 0o644)
	os.WriteFile(filepath.Join(root, "web", "web.kg"), []byte("@schema\npub type Metadata = {\n    tag String\n}\n\npub pure fn get(path: String) -> Metadata {\n    Metadata { tag: path }\n}\n"), 0o644)
	os.WriteFile(filepath.Join(root, "main.kg"), []byte("import web from app/web\n\n@web.get(\"/users/:id\")\nfn show() -> Int {\n    1\n}\n"), 0o644)
	std, _ := filepath.Abs("../../../../std")
	code, out, errOut := clitest.Execute(t, "--std", std, "schema", root)
	if code != 0 {
		t.Fatalf("schema: %s", errOut)
	}
	if strings.Contains(out, "unresolvedAttributes") {
		t.Errorf("resolved metadata must not set unresolvedAttributes:\n%s", out)
	}
	if !strings.Contains(out, "\"name\": \"web.get\"") {
		t.Errorf("missing resolved attribute:\n%s", out)
	}
}
