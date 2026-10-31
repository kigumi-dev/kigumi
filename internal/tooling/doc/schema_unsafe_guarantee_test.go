package doc_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/toolchain/driver"
	"kigumi/internal/tooling/doc"
)

// TestSchemaShowsUnsafeGuarantee checks that a type's `@unsafeGuarantee` reaches
// both the Markdown doc and the JSON schema's declaration signature, and changes it.
func TestSchemaShowsUnsafeGuarantee(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "sock"), 0o755)
	write := func(guarantee string) *driver.Module {
		src := "/// Safety: fd is a plain OS handle.\n" + guarantee + "\npub type Socket = resource {\n    fd i32\n}\n"
		if err := os.WriteFile(filepath.Join(root, "sock", "sock.kg"), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler()})
		if err != nil {
			t.Fatal(err)
		}
		return m
	}

	m := write("@unsafeGuarantee(Send)")
	page := doc.Module(m, doc.Options{})["sock"]
	if !strings.Contains(page, "@unsafeGuarantee(Send)") {
		t.Errorf("doc output missing the guarantee:\n%s", page)
	}

	res, err := driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	decls := doc.Declarations(m, res, "")
	var before string
	for _, d := range decls {
		if d.Name == "Socket" {
			before = d.Signature
		}
	}
	if !strings.Contains(before, "@unsafeGuarantee(Send)") {
		t.Errorf("schema signature missing the guarantee: %q", before)
	}

	m = write("@unsafeGuarantee(Send, Sync)")
	res, err = driver.Check(m)
	if err != nil {
		t.Fatal(err)
	}
	if res.HasErrors() {
		t.Fatalf("check errors:\n%s", res.Render())
	}
	var after string
	for _, d := range doc.Declarations(m, res, "") {
		if d.Name == "Socket" {
			after = d.Signature
		}
	}
	if after == before {
		t.Errorf("schema signature did not change when the guarantee did: %q", after)
	}
	if !strings.Contains(after, "@unsafeGuarantee(Send, Sync)") {
		t.Errorf("schema signature missing the updated guarantee: %q", after)
	}
}
