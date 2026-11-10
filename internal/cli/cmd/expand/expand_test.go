package expand_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/cli/clitest"
)

const expandSrc = `
pub type Point = {
    pub x Int
    pub y Int
}

fn same(a: Point, b: Point) -> Bool {
    a == b
}
`

func writeModule(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "mod.kg"), clitest.Manifest("app", ""), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.kg"), []byte(expandSrc), 0o644); err != nil {
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

// TestExpand prints the generated source with the declaration it came from.
func TestExpand(t *testing.T) {
	root := writeModule(t)
	code, out, errOut := clitest.Execute(t, "--std", stdRoot(t), "expand", root)
	if code != 0 {
		t.Fatalf("expand: %s", errOut)
	}
	for _, want := range []string{
		"// app/proto.derived.kg sha256:",
		"//   derive:Eq <- app#type:Point",
		"fn Point.equals(",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
}

// TestExpandJSON emits the versioned machine-readable form.
func TestExpandJSON(t *testing.T) {
	root := writeModule(t)
	code, out, errOut := clitest.Execute(t, "--std", stdRoot(t), "expand", "--format", "json", root)
	if code != 0 {
		t.Fatalf("expand --format json: %s", errOut)
	}
	var envelope struct {
		FormatVersion int    `json:"formatVersion"`
		Kind          string `json:"kind"`
		Data          struct {
			Files []struct {
				Package string `json:"package"`
				Name    string `json:"name"`
				Text    string `json:"text"`
				Sources []struct {
					Generator string `json:"generator"`
					Origin    struct {
						ID string `json:"id"`
					} `json:"origin"`
				} `json:"sources"`
			} `json:"files"`
		} `json:"data"`
	}
	if err := json.Unmarshal([]byte(out), &envelope); err != nil {
		t.Fatalf("expand JSON: %v\n%s", err, out)
	}
	if envelope.Kind != "expand" || envelope.FormatVersion != 1 {
		t.Errorf("envelope = %+v, want kind expand formatVersion 1", envelope)
	}
	if len(envelope.Data.Files) != 1 {
		t.Fatalf("files = %+v, want one", envelope.Data.Files)
	}
	f := envelope.Data.Files[0]
	if f.Package != "app" || f.Name != "proto.derived.kg" || !strings.Contains(f.Text, "fn Point.equals(") {
		t.Errorf("file = %+v", f)
	}
	if len(f.Sources) != 1 || f.Sources[0].Generator != "derive:Eq" || f.Sources[0].Origin.ID != "app#type:Point" {
		t.Errorf("sources = %+v, want derive:Eq from app#type:Point", f.Sources)
	}
}

// TestExpandRejectsUnknownFormat keeps the flag from silently falling back.
func TestExpandRejectsUnknownFormat(t *testing.T) {
	root := writeModule(t)
	code, _, errOut := clitest.Execute(t, "--std", stdRoot(t), "expand", "--format", "yaml", root)
	if code != 2 {
		t.Fatalf("exit = %d, want 2; stderr: %s", code, errOut)
	}
	if !strings.Contains(errOut, "--format") {
		t.Errorf("stderr does not name the flag: %s", errOut)
	}
}
