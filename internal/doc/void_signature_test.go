package doc_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/doc"
	"kigumi/internal/driver"
)

func TestVoidSignatureForms(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "tasks"), 0o755)
	src := "pub fn cleanup() {}\n\npub fn explicit() -> Void {}\n\npub fn run() ! {}\n\npub fn runLong() -> Void! {}\n"
	os.WriteFile(filepath.Join(root, "tasks", "tasks.kg"), []byte(src), 0o644)
	m, err := driver.LoadModule(root, driver.LoadOptions{Compiler: testCompiler()})
	if err != nil {
		t.Fatal(err)
	}
	page := doc.Module(m, doc.Options{})["tasks"]
	for _, want := range []string{"pub fn cleanup()\n", "pub fn explicit() -> Void\n", "pub fn run() !\n", "pub fn runLong() -> Void!\n"} {
		if !strings.Contains(page, want) {
			t.Errorf("missing %q in:\n%s", want, page)
		}
	}
}
