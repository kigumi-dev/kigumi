package doc_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kigumi/internal/tooling/doc"
)

// A CommonMark destination may hold balanced parentheses (and a backslash
// escape); the href the site emits and the validator that reads it back must
// cut the target at the same place.
func TestHTMLSiteKeepsBalancedParenTargets(t *testing.T) {
	pages := map[string]string{
		"main":       "# main\n\nSee [wiki](https://en.wikipedia.org/wiki/Foo_(bar)) and [notes](notes_(v2).md) and [intro](notes_(v2).md#intro).\n",
		"notes_(v2)": "# notes_(v2)\n\n## Intro\n\nBack to [main](main.md).\n",
	}
	site := renderSite(t, pages, "demo")
	page := site["main.html"]
	for _, want := range []string{
		`href="https://en.wikipedia.org/wiki/Foo_(bar)"`,
		`href="notes_(v2).html"`,
		`href="notes_(v2).html#intro"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("missing %q in:\n%s", want, page)
		}
	}
	if strings.Contains(page, "Foo_(bar)</a>") {
		t.Errorf("the trailing parenthesis leaked into the body:\n%s", page)
	}
	if err := doc.Publish(t.TempDir(), toBytes(site), doc.ValidateSite); err != nil {
		t.Errorf("a balanced-paren site must validate: %v", err)
	}
}

// A relative target naming no published page is rendered as text: the site
// never ships an href that leads nowhere, and validation of the pages the
// generator does write stays strict.
func TestHTMLSiteUnpublishedRelativeLinkStaysText(t *testing.T) {
	site := renderSite(t, map[string]string{"a": "# a\n\nclick [tab](notes_(v2).md) for more\n"}, "demo")
	page := site["a.html"]
	if strings.Contains(page, `href="notes_(v2).md"`) {
		t.Errorf("an unpublished target must not become an href:\n%s", page)
	}
	if !strings.Contains(page, "<p>click tab for more</p>") {
		t.Errorf("the label should stay as text:\n%s", page)
	}
	if err := doc.Publish(t.TempDir(), toBytes(site), doc.ValidateSite); err != nil {
		t.Errorf("Publish: %v", err)
	}
}

// The Markdown output keeps the authored link; its validator reads the same
// balanced destination, so a missing target is reported and a present one is not.
func TestValidateMarkdownSiteBalancedParenTarget(t *testing.T) {
	dir := t.TempDir()
	page := filepath.Join(dir, "a.md")
	if err := os.WriteFile(page, []byte("click [tab](notes_(v2).md) for more\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "notes_(v2).md")
	if err := os.WriteFile(target, []byte("# notes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := doc.ValidateMarkdownSite(dir); err != nil {
		t.Fatalf("a balanced-paren link to a written page must validate: %v", err)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	err := doc.ValidateMarkdownSite(dir)
	if err == nil || !strings.Contains(err.Error(), "notes_(v2).md") {
		t.Errorf("missing target: err = %v, want it named", err)
	}
}
