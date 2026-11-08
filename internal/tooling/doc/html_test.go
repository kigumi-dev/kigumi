package doc_test

import (
	"strconv"
	"strings"
	"testing"

	"kigumi/internal/tooling/doc"
)

// renderSite renders a site or fails the test; template parse/render errors
// should never be swallowed by a test.
func renderSite(t *testing.T, pages map[string]string, title string) map[string]string {
	t.Helper()
	site, err := doc.HTML(pages, title)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	return site
}

// The HTML site has an index, one page per package, and the Markdown
// forms the renderer emits turned into their HTML, with kigumi code blocks
// tokenized by the compiler lexer.
func TestHTMLSite(t *testing.T) {
	pages := map[string]string{"shapes": "# shapes\n\n## Types\n\n### Square\n\n```kigumi\npub type Square = {\n    pub side Int\n}\n```\n\nAxis-aligned `square` with <side>.\n"}
	site := renderSite(t, pages, "demo")
	index, page := site["index.html"], site["shapes.html"]
	for _, want := range []string{
		`<title>demo</title>`,
		`<p class="eyebrow">API reference</p>`,
		`<a class="package-card" href="shapes.html" data-card-search="shapes">`,
		`<span class="package-card-name">shapes</span>`,
	} {
		if !strings.Contains(index, want) {
			t.Errorf("index missing %q in:\n%s", want, index)
		}
	}
	for _, want := range []string{
		`<h2 id="types"><a class="heading-anchor" href="#types" aria-label="Permalink: Types" tabindex="-1">#</a>Types</h2>`,
		`<div class="code-block" data-lang="kigumi"><pre><code class="language-kigumi"><span class="tok-kw">pub</span> <span class="tok-kw">type</span> Square = {` + "\n" + `    <span class="tok-kw">pub</span> side Int` + "\n" + `}` + "\n" + `</code></pre>` + "\n" + `</div>`,
		"<p>Axis-aligned <code>square</code> with &lt;side&gt;.</p>",
		`<nav class="package-nav" aria-label="Packages">`,
		`<a class="nav-link is-current" aria-current="page" href="shapes.html" title="shapes">`,
		`<aside class="toc" aria-label="On this page">`,
		`<a href="#types">Types</a>`,
		`<a class="skip-link" href="#kg-content">Skip to content</a>`,
		`data-theme-toggle`,
		`id="kg-sidebar"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("missing %q in:\n%s", want, page)
		}
	}
}

// The shell keeps the page readable without JS: landmarks, a package tree,
// a TOC that matches the heading ids, and relative links at every depth.
func TestHTMLSiteShell(t *testing.T) {
	pages := map[string]string{
		"root": "# root\n\n## Section\n\nText.\n",
		"a/b":  "# a/b\n\n## Types\n\n### T\n\n```kigumi\npub fn x() {}\n```\n",
	}
	site := renderSite(t, pages, "demo")
	page := site["a/b.html"]
	for _, want := range []string{
		`<a class="skip-link" href="#kg-content">Skip to content</a>`,
		`<a class="nav-link is-current" aria-current="page" href="../a/b.html" title="a/b">`,
		`<span class="nav-label">b</span>`,
		`<div class="nav-group">`,
		`<p class="nav-group-title" title="a"><span>a</span><span class="nav-count">1</span></p>`,
		`<details class="toc-mobile"><summary>On this page</summary>`,
		`data-search="a/b"`,
		`<p class="toc-title">On this page</p>`,
		`<li class="toc-depth-2"><a href="#types">Types</a></li>`,
		`<li class="toc-depth-3"><a href="#t">T</a></li>`,
		`<a class="brand" href="../index.html">`,
		`<a href="../index.html">All packages</a>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("a/b.html missing %q in:\n%s", want, page)
		}
	}
	if err := doc.Publish(t.TempDir(), toBytes(site), doc.ValidateSite); err != nil {
		t.Errorf("Publish: %v", err)
	}
}

func TestHTMLSiteTOCCollapsesLongSymbolLists(t *testing.T) {
	var b strings.Builder
	b.WriteString("# big\n\n## Types\n")
	for i := range 41 {
		b.WriteString("\n### T" + strconv.Itoa(i) + "\n\nText.\n")
	}
	site := renderSite(t, map[string]string{"big": b.String()}, "demo")
	page := site["big.html"]
	if strings.Contains(page, `class="toc-depth-3"`) {
		t.Errorf("a long symbol list should collapse the TOC to its sections:\n%s", page)
	}
	if !strings.Contains(page, `<li class="toc-depth-2"><a href="#types">Types</a></li>`) {
		t.Errorf("TOC lost the section entry:\n%s", page)
	}
	if err := doc.Publish(t.TempDir(), toBytes(site), doc.ValidateSite); err != nil {
		t.Errorf("Publish: %v", err)
	}
}

func TestHTMLSiteEscapesTemplateData(t *testing.T) {
	site := renderSite(t, map[string]string{"safe": "# safe\n"}, `<script>alert(1)</script>`)
	for _, name := range []string{"index.html", "safe.html"} {
		page := site[name]
		if strings.Contains(page, "<script>alert(1)</script>") {
			t.Errorf("%s contains the raw title:\n%s", name, page)
		}
		if !strings.Contains(page, "&lt;script&gt;alert(1)&lt;/script&gt;") {
			t.Errorf("%s missing the escaped title:\n%s", name, page)
		}
	}
}

func TestHTMLSiteEmpty(t *testing.T) {
	site := renderSite(t, nil, "demo")
	index := site["index.html"]
	if !strings.Contains(index, `<p class="empty">No documented packages.</p>`) {
		t.Errorf("empty site index missing the empty message:\n%s", index)
	}
	if strings.Contains(index, `<a class="package-card"`) {
		t.Errorf("empty site should not render package cards:\n%s", index)
	}
}

func TestHTMLSiteNestedNav(t *testing.T) {
	pages := map[string]string{
		"root":     "# root\n\nSee [a/x](a/x.md) and [a/b/leaf](a/b/leaf.md).\n",
		"a/x":      "# a/x\n\nSee [root](root.md).\n",
		"a/b/leaf": "# a/b/leaf\n\nSee [root](root.md) and [a/x](a/x.md).\n",
	}
	site := renderSite(t, pages, "demo")

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
	site := renderSite(t, pages, "demo")
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
	site := renderSite(t, pages, "demo")
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
	site := renderSite(t, pages, "demo")
	if !strings.Contains(site["index.html"], "real content") {
		t.Errorf("index.html should be the index package's own page:\n%s", site["index.html"])
	}
	list, ok := site["index-2.html"]
	if !ok {
		t.Fatalf("expected a fallback list page, got keys %v", keys(site))
	}
	if !strings.Contains(list, `href="other.html"`) || !strings.Contains(list, `>other</span>`) {
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

func TestHTMLSiteNonASCIIHeadings(t *testing.T) {
	site := renderSite(t, map[string]string{"u8": "# u8\n\n## 使い方\n\nCall it.\n\n## Café notes\n\nMore.\n"}, "demo")
	page := site["u8.html"]
	for _, id := range []string{"使い方", "café-notes"} {
		if !strings.Contains(page, `<a href="#`+id+`">`) || !strings.Contains(page, `id="`+id+`"`) {
			t.Errorf("TOC link and heading id disagree for %q:\n%s", id, page)
		}
	}
	if err := doc.Publish(t.TempDir(), toBytes(site), doc.ValidateSite); err != nil {
		t.Errorf("Publish: %v", err)
	}
}

func TestHTMLSiteDevanagariHeadingAndLinks(t *testing.T) {
	md := "# hi\n\n## हिन्दी\n\nSee [the heading](#हिन्दी) and [encoded](#%E0%A4%B9%E0%A4%BF%E0%A4%A8%E0%A5%8D%E0%A4%A6%E0%A5%80).\n"
	site := renderSite(t, map[string]string{"hi": md}, "demo")
	page := site["hi.html"]
	if !strings.Contains(page, `id="हिन्दी"`) {
		t.Errorf("combining marks were dropped from the heading id:\n%s", page)
	}
	if err := doc.Publish(t.TempDir(), toBytes(site), doc.ValidateSite); err != nil {
		t.Errorf("Publish: %v", err)
	}
}

func TestHTMLSiteHeadingIDsStayUnique(t *testing.T) {
	site := renderSite(t, map[string]string{"dup": "# dup\n\n## a\n\n## a\n\n## a-2\n\n## kg-content\n"}, "demo")
	for _, id := range []string{"a", "a-2", "a-2-2", "kg-content-2"} {
		if strings.Count(site["dup.html"], `<h2 id="`+id+`">`) != 1 {
			t.Errorf("heading id %q missing or repeated:\n%s", id, site["dup.html"])
		}
	}
	if err := doc.Publish(t.TempDir(), toBytes(site), doc.ValidateSite); err != nil {
		t.Errorf("Publish: %v", err)
	}
}
