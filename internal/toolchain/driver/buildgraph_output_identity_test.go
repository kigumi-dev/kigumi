package driver_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuildWriteRejectsDuplicateName checks that Builder.write rejects an output name already claimed by an earlier generated file, as runTool's outputs do.
func TestBuildWriteRejectsDuplicateName(t *testing.T) {
	t.Parallel()
	root, out := t.TempDir(), t.TempDir()
	program := "import build from std/build\n\nfn configure(b: build.Builder) -> Void! {\n" +
		"    let a = b.write(\"gen.txt\", \"first\")?\n" +
		"    let b2 = b.write(\"gen.txt\", \"second\")?\n" +
		"}\n"
	err := buildWith(t, root, out, program)
	if err == nil || !strings.Contains(err.Error(), "collides") {
		t.Fatalf("expected a duplicate-output refusal, got %v", err)
	}
}

// TestBuildWriteRejectsNameOfToolOutputFirst checks that a write can't silently take over a name a tool step already produces.
func TestBuildWriteRejectsNameOfToolOutputFirst(t *testing.T) {
	t.Parallel()
	root, out := t.TempDir(), t.TempDir()
	program := "import build from std/build\n\nfn configure(b: build.Builder) -> Void! {\n" +
		"    let cc = b.toolchain()\n" +
		"    let outs = b.runTool(cc, \"cc\", Array.empty[String](), Array.empty[build.FilePath](), Array.of(\"gen.txt\"), Array.empty[build.Secret]())?\n" +
		"    let w = b.write(\"gen.txt\", \"second\")?\n" +
		"}\n"
	err := buildWith(t, root, out, program)
	if err == nil || !strings.Contains(err.Error(), "collides") {
		t.Fatalf("expected a duplicate-output refusal, got %v", err)
	}
}

// TestBuildArtifactRejectsOutputNameCollision covers an artifact's own
// output name colliding with an already-declared generated file.
func TestBuildArtifactRejectsOutputNameCollision(t *testing.T) {
	t.Parallel()
	root, out := t.TempDir(), t.TempDir()
	program := "import build from std/build\n\nfn configure(b: build.Builder) -> Void! {\n" +
		"    let gen = b.write(\"app\", \"not a binary\")?\n" +
		"    let a = b.executable(\"app\", \".\", b.hostTarget())?\n" +
		"}\n"
	err := buildWith(t, root, out, program)
	if err == nil || !strings.Contains(err.Error(), "collides") {
		t.Fatalf("expected a duplicate-output refusal, got %v", err)
	}
}

// TestBuildWriteMayReuseAnInputFileName checks that a module-root input and a generated output of the same name live in different directories.
func TestBuildWriteMayReuseAnInputFileName(t *testing.T) {
	t.Parallel()
	root, out := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "notes.txt"), []byte("input"), 0o644); err != nil {
		t.Fatal(err)
	}
	program := "import build from std/build\n\nfn configure(b: build.Builder) -> Void! {\n" +
		"    let input = b.file(\"notes.txt\")\n" +
		"    let gen = b.write(\"notes.txt\", \"generated\")?\n" +
		"}\n"
	if err := buildWith(t, root, out, program); err != nil {
		t.Fatalf("a literal input blocked a generated output: %v", err)
	}
}
