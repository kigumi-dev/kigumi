package doc_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/doc"
	"kigumi/internal/driver"
)

const surfaceSrc = `import text from std/text

/// A counter.
pub static mut hits: i32 = 0

extern(C) {
    /// Absolute value.
    pub fn abs(x: i32) -> i32
    fn labs(x: i64) -> i64
}

export(C) {
    /// Exported double.
    pub fn twice(x: i32) -> i32 {
        x * 2
    }
}

/// Re-exported.
pub import text2 from std/text

/// A record.
pub type Rec = {
    /// The public name.
    pub name String
    secret Int
    pub mut count Int
}
`

func surfacePage(t *testing.T, opts doc.Options) string {
	t.Helper()
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "lib"), 0o755)
	os.WriteFile(filepath.Join(root, "lib", "lib.kg"), []byte(surfaceSrc), 0o644)
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler()})
	if err != nil {
		t.Fatal(err)
	}
	return doc.Module(m, opts)["lib"]
}

func TestDocCoversStaticAbiAndReexport(t *testing.T) {
	page := surfacePage(t, doc.Options{})
	for _, want := range []string{
		"## Statics", "pub static mut hits: i32 = 0", "A counter.",
		"extern(C) {\n    pub fn abs(x: i32) -> i32\n}", "Absolute value.",
		"export(C) {\n    pub fn twice(x: i32) -> i32\n}", "Exported double.",
		"## Re-exports", "pub import text2 from std/text",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("missing %q in:\n%s", want, page)
		}
	}
	for _, unwanted := range []string{"labs", "import text from", "secret"} {
		if strings.Contains(page, unwanted) {
			t.Errorf("private %q rendered:\n%s", unwanted, page)
		}
	}
}

func TestDocFieldDocAndPrivateFields(t *testing.T) {
	page := surfacePage(t, doc.Options{})
	want := "pub type Rec = {\n    /// The public name.\n    pub name String\n    pub mut count Int\n    // private fields omitted\n}"
	if !strings.Contains(page, want) {
		t.Errorf("missing public-only record in:\n%s", page)
	}
	all := surfacePage(t, doc.Options{All: true})
	for _, want := range []string{"    secret Int", "fn labs(x: i64) -> i64", "/// The public name."} {
		if !strings.Contains(all, want) {
			t.Errorf("--all missing %q in:\n%s", want, all)
		}
	}
	if strings.Contains(all, "import text from") {
		t.Errorf("--all rendered a private import:\n%s", all)
	}
}
