package doc_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
	"kigumi/internal/tooling/doc"
)

// AUD-26: a brace in a doc comment is comment text. It must not extend a
// declaration head past its closing brace, swallow a following declaration
// into the schema signature, or drop later public fields from the doc.
func TestDocCommentBracesKeepDeclarationBounds(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "braces"), 0o755)
	src := `pub type R = {
    pub first Int
    /// uses the { placeholder
    hidden Int
    pub shown String
}

pub type T = {
    /// tag { for docs
    pub field Int
}

pub fn after() -> Int {
    1
}
`
	if err := os.WriteFile(filepath.Join(root, "braces", "braces.kg"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler()})
	if err != nil {
		t.Fatal(err)
	}
	page := doc.Module(m, doc.Options{})["braces"]
	for _, want := range []string{
		"```kigumi\npub type R = {\n    pub first Int\n    pub shown String\n    // private fields omitted\n}\n```",
		"```kigumi\npub type T = {\n    /// tag { for docs\n    pub field Int\n}\n```",
	} {
		if !strings.Contains(page, want) {
			t.Errorf("missing %q in:\n%s", want, page)
		}
	}
	if strings.Contains(page, "hidden") {
		t.Errorf("private field rendered:\n%s", page)
	}

	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	decls := map[string]doc.Declaration{}
	for _, d := range doc.Declarations(m, res, "") {
		decls[d.Name] = d
	}
	for name, want := range map[string]string{
		"R":     "pub type R = {\n    pub first Int\n    /// uses the { placeholder\n    hidden Int\n    pub shown String\n}",
		"T":     "pub type T = {\n    /// tag { for docs\n    pub field Int\n}",
		"after": "pub fn after() -> Int",
	} {
		if got := decls[name].Signature; got != want {
			t.Errorf("%s signature =\n%s\nwant\n%s", name, got, want)
		}
	}
	var fields []string
	for _, f := range decls["R"].Fields {
		fields = append(fields, f.Name)
	}
	if got := strings.Join(fields, ","); got != "first,hidden,shown" {
		t.Errorf("R fields = %q", got)
	}
}
