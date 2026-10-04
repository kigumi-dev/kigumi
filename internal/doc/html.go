package doc

import (
	"html"
	"path"
	"sort"
	"strings"
)

// HTML renders the Markdown pages as a static site: one page per package
// plus index.html, styled inline so the output needs no other file. Every
// href is relative to the page that carries it, so the site works from any
// directory depth. The Markdown is our own (headings, fenced code, list
// items, paragraphs), so a small converter covers it.
func HTML(pages map[string]string, title string) map[string]string {
	paths := make([]string, 0, len(pages))
	for p := range pages {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	indexName := IndexFileName(paths, ".html", "index.html")
	out := map[string]string{}
	for _, p := range paths {
		selfDir := pageDir(p)
		nav := navHTML(paths, selfDir, p, indexName)
		out[p+".html"] = page(p+" - "+title, nav, markdownToHTML(pages[p], selfDir))
	}
	var index strings.Builder
	index.WriteString("<h1>" + html.EscapeString(title) + "</h1>\n<ul>\n")
	for _, p := range paths {
		index.WriteString("<li><a href=\"" + html.EscapeString(p) + ".html\">" + html.EscapeString(p) + "</a></li>\n")
	}
	index.WriteString("</ul>\n")
	out[indexName] = page(title, navHTML(paths, "", "", indexName), index.String())
	return out
}

// pageDir is the root-relative directory a page's file lives in; "" at the site root.
func pageDir(p string) string {
	if d := path.Dir(p); d != "." {
		return d
	}
	return ""
}

// relHref makes target (a root-relative site path, e.g. "std/array.html")
// relative to a page living in selfDir.
func relHref(selfDir, target string) string {
	if selfDir == "" {
		return target
	}
	return strings.Repeat("../", strings.Count(selfDir, "/")+1) + target
}

func navHTML(paths []string, selfDir, current, indexName string) string {
	var sb strings.Builder
	sb.WriteString("<nav>")
	writeNavLink(&sb, relHref(selfDir, indexName), "Packages", current == "")
	for _, p := range paths {
		writeNavLink(&sb, relHref(selfDir, p+".html"), p, p == current)
	}
	sb.WriteString("</nav>\n")
	return sb.String()
}

func writeNavLink(sb *strings.Builder, href, text string, isCurrent bool) {
	sb.WriteString("<a href=\"" + html.EscapeString(href) + "\"")
	if isCurrent {
		sb.WriteString(" aria-current=\"page\"")
	}
	sb.WriteString(">" + html.EscapeString(text) + "</a>\n")
}

const style = `body{margin:0;font:15px/1.5 system-ui,sans-serif;color:#1f2328;background:#fff}
main{max-width:56rem;margin:0 auto;padding:1.5rem 2rem 4rem}
nav{border-bottom:1px solid #d0d7de;padding:.5rem 2rem;display:flex;flex-wrap:wrap;gap:.25rem 1rem;font-size:14px}
a{color:#0969da;text-decoration:none}a:hover{text-decoration:underline}
a[aria-current="page"]{color:inherit;font-weight:600;text-decoration:none}
h1{font-size:1.8rem}h2{border-bottom:1px solid #d0d7de;padding-bottom:.25rem;margin-top:2rem}
h3{margin-top:1.5rem}h4{margin:1rem 0 0 1rem}
pre{background:#f6f8fa;border-radius:6px;padding:.75rem 1rem;overflow-x:auto}
code{font-family:ui-monospace,SFMono-Regular,Menlo,monospace;font-size:.92em}
p code{background:#f6f8fa;border-radius:4px;padding:0 .25em}
.tok-kw{color:#cf222e}.tok-lit{color:#0a3069}.tok-com{color:#6e7781}
@media(prefers-color-scheme:dark){body{color:#e6edf3;background:#0d1117}nav{border-color:#30363d}h2{border-color:#30363d}pre,p code{background:#161b22}a{color:#58a6ff}
.tok-kw{color:#ff7b72}.tok-lit{color:#a5d6ff}.tok-com{color:#8b949e}}`

func page(title, nav, body string) string {
	return "<!doctype html>\n<html lang=\"en\">\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">\n<title>" +
		html.EscapeString(title) + "</title>\n<style>" + style + "</style>\n</head>\n<body>\n" + nav + "<main>\n" + body + "</main>\n</body>\n</html>\n"
}
