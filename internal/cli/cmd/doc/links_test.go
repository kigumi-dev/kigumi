package doc_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"kigumi/internal/cli/clitest"
)

// A doc comment link whose target holds balanced parentheses keeps its whole
// destination in both outputs, and the site publishes instead of rejecting the
// page as a broken link.
func TestDocPublishesBalancedParenLinks(t *testing.T) {
	mod := t.TempDir()
	clitest.WriteFiles(t, mod, map[string]string{
		"mod.kg":          "Module {\n    name: \"parens\"\n    kigumi: \"0.1\"\n}\n",
		"a/a.kg":          "/// see [wiki](https://en.wikipedia.org/wiki/Foo_(bar)) and [notes](notes_(v2).md) for more\npub fn f() {}\n",
		"notes_(v2)/n.kg": "/// the v2 notes\npub fn g() {}\n",
	}, time.Now().Add(-time.Hour))

	site := t.TempDir()
	if code, _, errOut := clitest.Execute(t, "doc", "--html", "-o", site, mod); code != 0 {
		t.Fatalf("doc --html -o: exit %d\n%s", code, errOut)
	}
	page, err := os.ReadFile(filepath.Join(site, "parens", "a.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`href="https://en.wikipedia.org/wiki/Foo_(bar)"`,
		`href="../parens/notes_(v2).html"`,
	} {
		if !strings.Contains(string(page), want) {
			t.Errorf("missing %q in:\n%s", want, page)
		}
	}

	md := t.TempDir()
	if code, _, errOut := clitest.Execute(t, "doc", "-o", md, mod); code != 0 {
		t.Fatalf("doc -o: exit %d\n%s", code, errOut)
	}
	text, err := os.ReadFile(filepath.Join(md, "parens", "a.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(text), "[notes](notes_(v2).md)") {
		t.Errorf("the Markdown output should keep the link as written:\n%s", text)
	}
}
