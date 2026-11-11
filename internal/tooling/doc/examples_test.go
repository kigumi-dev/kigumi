package doc_test

import (
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/tooling/doc"
)

func TestExamplesCatalogMarkdown(t *testing.T) {
	root, _ := filepath.Abs("../../../examples")
	md, err := doc.ExamplesCatalogMarkdown(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# Examples",
		"### tour",
		"### data_tools",
		"oracle: `build-graph`",
		"covers: `",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("missing %q in:\n%s", want, md)
		}
	}
	site := renderSite(t, map[string]string{"examples": md}, "Examples")
	if err := doc.Publish(t.TempDir(), toBytes(site), doc.ValidateSite); err != nil {
		t.Errorf("generated catalog page should validate: %v", err)
	}
}
