package doc_test

import (
	"strings"
	"testing"

	"kigumi/internal/doc"
)

// The HTML site has an index, one page per package, and the Markdown
// forms the renderer emits turned into their HTML, with kigumi code blocks
// tokenized by the compiler lexer.
func TestHTMLSite(t *testing.T) {
	pages := map[string]string{"shapes": "# shapes\n\n## Types\n\n### Square\n\n```kigumi\npub type Square = {\n    pub side Int\n}\n```\n\nAxis-aligned `square` with <side>.\n"}
	site := doc.HTML(pages, "demo")
	index, page := site["index.html"], site["shapes.html"]
	if !strings.Contains(index, `<a href="shapes.html">shapes</a>`) || !strings.Contains(index, "<title>demo</title>") {
		t.Errorf("index:\n%s", index)
	}
	for _, want := range []string{
		`<h2 id="types">Types</h2>`,
		`<pre><code class="language-kigumi"><span class="tok-kw">pub</span> <span class="tok-kw">type</span> Square = {` + "\n" + `    <span class="tok-kw">pub</span> side Int` + "\n" + `}` + "\n" + `</code></pre>`,
		"<p>Axis-aligned <code>square</code> with &lt;side&gt;.</p>",
		`<nav><a href="index.html">Packages</a>`,
		`<a href="shapes.html" aria-current="page">shapes</a>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("missing %q in:\n%s", want, page)
		}
	}
}

func TestHTMLSiteNestedNav(t *testing.T) {
	pages := map[string]string{
		"root":     "# root\n\nSee [a/x](a/x.md) and [a/b/leaf](a/b/leaf.md).\n",
		"a/x":      "# a/x\n\nSee [root](root.md).\n",
		"a/b/leaf": "# a/b/leaf\n\nSee [root](root.md) and [a/x](a/x.md).\n",
	}
	site := doc.HTML(pages, "demo")

	for _, want := range []string{`href="a/x.html"`, `href="a/b/leaf.html"`, `href="index.html"`} {
		if !strings.Contains(site["root.html"], want) {
			t.Errorf("root.html missing %q:\n%s", want, site["root.html"])
		}
	}
	for _, want := range []string{`href="../index.html"`, `href="../root.html"`, `href="../a/x.html"`, `href="../a/b/leaf.html"`} {
		if !strings.Contains(site["a/x.html"], want) {
			t.Errorf("a/x.html missing %q:\n%s", want, site["a/x.html"])
		}
	}
	for _, want := range []string{`href="../../index.html"`, `href="../../root.html"`, `href="../../a/x.html"`} {
		if !strings.Contains(site["a/b/leaf.html"], want) {
			t.Errorf("a/b/leaf.html missing %q:\n%s", want, site["a/b/leaf.html"])
		}
	}

	dir := t.TempDir()
	if err := doc.Publish(dir, toBytes(site), doc.ValidateSite); err != nil {
		t.Fatalf("Publish: %v", err)
	}
}

func TestHTMLSiteUniqueAnchors(t *testing.T) {
	pages := map[string]string{"math": "# math\n\n## Functions\n\n### min\n\n```kigumi\npub fn min(a Int, b Int) -> Int\n```\n\n### min\n\n```kigumi\npub fn min(a Float, b Float) -> Float\n```\n"}
	site := doc.HTML(pages, "demo")
	page := site["math.html"]
	if !strings.Contains(page, `id="min"`) || !strings.Contains(page, `id="min-2"`) {
		t.Errorf("expected distinct min/min-2 anchors:\n%s", page)
	}
	if err := doc.Publish(t.TempDir(), toBytes(site), doc.ValidateSite); err != nil {
		t.Errorf("duplicate-id site should still validate once anchors are unique: %v", err)
	}
}

func TestValidateSiteCatchesBrokenLink(t *testing.T) {
	broken := map[string][]byte{"index.html": []byte(`<a href="missing.html">x</a>`)}
	if err := doc.Publish(t.TempDir(), broken, doc.ValidateSite); err == nil {
		t.Error("expected a broken link to fail validation")
	}
}

func TestValidateSiteCatchesDuplicateID(t *testing.T) {
	dup := map[string][]byte{"index.html": []byte(`<h2 id="x">a</h2><h2 id="x">b</h2>`)}
	if err := doc.Publish(t.TempDir(), dup, doc.ValidateSite); err == nil {
		t.Error("expected a duplicate id to fail validation")
	}
}

func TestHTMLSiteLinkClassification(t *testing.T) {
	pages := map[string]string{
		"a/page": "# a/page\n\n" +
			"[ex](https://example.com) " +
			"[mail](mailto:x@example.com) " +
			"[bad](javascript:alert(1)) " +
			"[frag](#section) " +
			"[other](a/other.md) " +
			"[otherfrag](a/other.md#section)\n",
		"a/other": "# a/other\n",
	}
	site := doc.HTML(pages, "demo")
	page := site["a/page.html"]
	for _, want := range []string{
		`href="https://example.com"`,
		`href="mailto:x@example.com"`,
		`href="#section"`,
		`href="../a/other.html"`,
		`href="../a/other.html#section"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("missing %q in:\n%s", want, page)
		}
	}
	if strings.Contains(page, "javascript:") {
		t.Errorf("javascript: URL must not reach the page:\n%s", page)
	}
	if !strings.Contains(page, "bad") {
		t.Errorf("label of a dropped link should still render as text:\n%s", page)
	}
}

func TestHTMLSiteIndexNameCollision(t *testing.T) {
	pages := map[string]string{"index": "# index\n\nreal content\n", "other": "# other\n"}
	site := doc.HTML(pages, "demo")
	if !strings.Contains(site["index.html"], "real content") {
		t.Errorf("index.html should be the index package's own page:\n%s", site["index.html"])
	}
	list, ok := site["index-2.html"]
	if !ok {
		t.Fatalf("expected a fallback list page, got keys %v", keys(site))
	}
	if !strings.Contains(list, `<a href="other.html">other</a>`) {
		t.Errorf("fallback list page missing package link:\n%s", list)
	}
	if !strings.Contains(site["other.html"], `href="index-2.html"`) {
		t.Errorf("nav Packages link should point at the fallback list page:\n%s", site["other.html"])
	}
	if err := doc.Publish(t.TempDir(), toBytes(site), doc.ValidateSite); err != nil {
		t.Errorf("Publish: %v", err)
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func toBytes(site map[string]string) map[string][]byte {
	out := make(map[string][]byte, len(site))
	for k, v := range site {
		out[k] = []byte(v)
	}
	return out
}
